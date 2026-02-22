package com.example.ordersservice.config.jwt;

import io.jsonwebtoken.Jwts;
import io.jsonwebtoken.SignatureAlgorithm;
import io.jsonwebtoken.security.Keys;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.web.reactive.function.client.ClientRequest;
import org.springframework.web.reactive.function.client.ExchangeFilterFunction;
import org.springframework.web.reactive.function.client.WebClient;

import java.nio.charset.StandardCharsets;
import java.security.Key;
import java.util.Date;

@Slf4j
@Configuration
@Profile("jwt")
public class JwtWebClientConfig {

    @Value("${app.security.jwt-secret}")
    private String jwtSecret;

    @Value("${app.services.payments-url}")
    private String paymentsServiceUrl;

    @Bean
    public WebClient paymentWebClient() {
        return WebClient.builder()
                .baseUrl(paymentsServiceUrl)
                .filter(jwtAuthHeaderFilter())
                .filter((request, next) -> {
                    log.info(">>> SENDING REQUEST TO: " + request.url());
                    return next.exchange(request);
                })
                .build();
    }

    private ExchangeFilterFunction jwtAuthHeaderFilter() {
        return (request, next) -> {
            String token = generateJwtToken();
            ClientRequest newRequest = ClientRequest.from(request)
                    .header("Authorization", "Bearer " + token)
                    .build();
            return next.exchange(newRequest);
        };
    }

    private String generateJwtToken() {
        byte[] keyBytes = jwtSecret.getBytes(StandardCharsets.UTF_8);
        Key key = Keys.hmacShaKeyFor(keyBytes);

        return Jwts.builder()
                .setSubject("orders-service")
                .setIssuedAt(new Date())
                .setExpiration(new Date(System.currentTimeMillis() + 300000))
                .signWith(key, SignatureAlgorithm.HS256)
                .compact();
    }
}
