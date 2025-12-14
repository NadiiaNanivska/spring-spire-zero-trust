package com.example.paymentsservice.config;

import org.springframework.boot.web.embedded.netty.NettyReactiveWebServerFactory;
import org.springframework.boot.web.server.WebServerFactoryCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration
public class TomcatServerCustomizer {

    @Bean
    public WebServerFactoryCustomizer<NettyReactiveWebServerFactory> tomcatCustomizer() {
        return factory -> {
            // SPIFFE Server SslContext goes here
        };
    }
}
