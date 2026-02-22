package com.example.paymentsservice.config.jwt;

import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.security.config.annotation.web.reactive.EnableWebFluxSecurity;
import org.springframework.security.config.web.server.SecurityWebFiltersOrder;
import org.springframework.security.config.web.server.ServerHttpSecurity;
import org.springframework.security.oauth2.jwt.NimbusReactiveJwtDecoder;
import org.springframework.security.oauth2.jwt.ReactiveJwtDecoder;
import org.springframework.security.web.server.SecurityWebFilterChain;

import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;

@Configuration
@EnableWebFluxSecurity
@Profile("jwt")
@Slf4j
public class JwtSecurityConfig {

    @Value("${app.security.jwt-secret}")
    private String jwtSecret;

    @Bean
    public SecurityWebFilterChain securityWebFilterChain(ServerHttpSecurity http) {
        return http
                .csrf(csrf -> csrf.disable())
                .formLogin(form -> form.disable())
                .httpBasic(basic -> basic.disable())
                .addFilterBefore((exchange, chain) -> {
                    String path = exchange.getRequest().getURI().getPath();
                    String authHeader = exchange.getRequest().getHeaders().getFirst("Authorization");
                    log.info(">>> [JWT-FILTER] Request to: {}", path);
                    if (authHeader != null && authHeader.startsWith("Bearer ")) {
                        log.info(">>> [JWT-FILTER] Token found: {}...", authHeader.substring(0, 15));
                    } else {
                        log.info(">>> [JWT-FILTER] NO TOKEN FOUND!");
                    }

                    return chain.filter(exchange);
                }, SecurityWebFiltersOrder.AUTHENTICATION)
                .oauth2ResourceServer(oauth2 -> oauth2
                        .jwt(jwt -> jwt.jwtDecoder(jwtDecoder()))
                )
                .authorizeExchange(exchanges -> exchanges
                        .pathMatchers("/api/payments/health").permitAll()
                        .pathMatchers("/actuator/prometheus").permitAll()
                        .pathMatchers("/actuator/health").permitAll()
                        .pathMatchers("/actuator/metrics").permitAll()
                        .anyExchange().authenticated()
                )
                .build();
    }

    @Bean
    public ReactiveJwtDecoder jwtDecoder() {
        byte[] secretKeyBytes = jwtSecret.getBytes(StandardCharsets.UTF_8);
        SecretKeySpec secretKey = new SecretKeySpec(secretKeyBytes, "HmacSHA256");
        return NimbusReactiveJwtDecoder.withSecretKey(secretKey).build();
    }
}
