package com.example.ordersservice.config;

import io.netty.handler.ssl.ClientAuth;
import io.netty.handler.ssl.JdkSslContext;
import io.netty.handler.ssl.SslContext;
import io.spiffe.exception.SocketEndpointAddressException;
import io.spiffe.exception.X509SourceException;
import io.spiffe.provider.SpiffeProvider;
import io.spiffe.provider.SpiffeSslContextFactory;
import io.spiffe.spiffeid.SpiffeId;
import io.spiffe.workloadapi.DefaultX509Source;
import io.spiffe.workloadapi.X509Source;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.client.reactive.ReactorClientHttpConnector;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.netty.http.client.HttpClient;

import javax.net.ssl.SSLContext;
import java.security.Security;
import java.util.Set;
import java.util.function.Supplier;

@Configuration
public class WebClientConfig {

    @Value("${app.security.payment-spiffe-id}")
    private String paymentSpiffeId;

    static {
        Security.addProvider(new SpiffeProvider());
    }

    @Bean(destroyMethod = "close")
    public X509Source x509Source()
            throws SocketEndpointAddressException, X509SourceException {

        DefaultX509Source.X509SourceOptions options =
                DefaultX509Source.X509SourceOptions.builder()
                        .spiffeSocketPath("unix:///run/spire/sockets/agent.sock")
                        .build();

        return DefaultX509Source.newSource(options);
    }

    @Bean
    public SSLContext spiffeClientSslContext(X509Source x509Source) {

        Supplier<Set<SpiffeId>> acceptedSpiffeIds =
                () -> Set.of(SpiffeId.parse(paymentSpiffeId));

        SpiffeSslContextFactory.SslContextOptions options =
                SpiffeSslContextFactory.SslContextOptions.builder()
                        .x509Source(x509Source)
                        .acceptedSpiffeIdsSupplier(acceptedSpiffeIds)
                        .build();

        try {
            return SpiffeSslContextFactory.getSslContext(options);
        } catch (Exception e) {
            throw new IllegalStateException("Failed to init SPIFFE client SSLContext", e);
        }
    }

    @Bean
    public WebClient paymentWebClient(SSLContext sslContext) {

        SslContext nettySslContext =
                new JdkSslContext(
                        sslContext,
                        true,
                        ClientAuth.REQUIRE
                );

        HttpClient httpClient = HttpClient.create()
                .secure(sslSpec -> sslSpec.sslContext(nettySslContext));

        return WebClient.builder()
                .clientConnector(new ReactorClientHttpConnector(httpClient))
                .build();
    }
}
