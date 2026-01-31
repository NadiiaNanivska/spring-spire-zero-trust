package com.example.paymentsservice.config;

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
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.web.embedded.netty.NettyReactiveWebServerFactory;
import org.springframework.boot.web.server.WebServerFactoryCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import javax.net.ssl.SSLContext;
import java.security.Security;
import java.util.Collections;
import java.util.Set;
import java.util.function.Supplier;

@Configuration
@Slf4j
public class SpiffeServerConfig {

    static {
        Security.addProvider(new SpiffeProvider());
    }

    @Bean(destroyMethod = "close")
    public X509Source x509Source() throws SocketEndpointAddressException, X509SourceException {
        DefaultX509Source.X509SourceOptions options = DefaultX509Source.X509SourceOptions.builder()
                .build();

        return DefaultX509Source.newSource(options);
    }

    @Bean
    public SSLContext spiffeSslContext(X509Source x509Source) {
        Supplier<Set<SpiffeId>> acceptedSpiffeIds = Collections::emptySet;

        SpiffeSslContextFactory.SslContextOptions options = SpiffeSslContextFactory.SslContextOptions.builder()
                .x509Source(x509Source)
                .acceptedSpiffeIdsSupplier(acceptedSpiffeIds)
                .build();

        try {
            return SpiffeSslContextFactory.getSslContext(options);
        } catch (Exception e) {
            throw new IllegalStateException("Failed to init SPIFFE SSLContext", e);
        }
    }

    @Bean
    public WebServerFactoryCustomizer<NettyReactiveWebServerFactory>
    nettyCustomizer(SSLContext sslContext) {

        return factory -> factory.addServerCustomizers(httpServer -> {

            SslContext nettySslContext =
                    new JdkSslContext(
                            sslContext,
                            true,
                            ClientAuth.REQUIRE
                    );

            log.info("SPIFFE mTLS enabled (Spring WebFlux + Netty)");

            return httpServer.secure(sslSpec -> sslSpec.sslContext(nettySslContext));
        });
    }
}