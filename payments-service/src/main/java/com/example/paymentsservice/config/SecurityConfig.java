package com.example.paymentsservice.config;

import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.web.reactive.EnableWebFluxSecurity;
import org.springframework.security.config.web.server.ServerHttpSecurity;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.userdetails.ReactiveUserDetailsService;
import org.springframework.security.core.userdetails.User;
import org.springframework.security.web.server.SecurityWebFilterChain;
import reactor.core.publisher.Mono;

import java.security.cert.X509Certificate;
import java.util.Collections;
import java.util.List;

@Slf4j
@Configuration
@EnableWebFluxSecurity
public class SecurityConfig {

    @Bean
    public SecurityWebFilterChain securityWebFilterChain(ServerHttpSecurity http) {
        return http
                .csrf(csrf -> csrf.disable())
                .formLogin(form -> form.disable())
                .httpBasic(basic -> basic.disable())
                .x509(x509 -> x509
                        .principalExtractor(cert -> {
                            String spiffeId = extractSpiffeId(cert);

                            if (spiffeId != null) {
                                log.info("Security: Authorized via SPIFFE ID: {}", spiffeId);
                                return spiffeId;
                            }

                            String subjectDn = cert.getSubjectX500Principal().getName();
                            log.info("Security: Authorized via Subject DN: {}", subjectDn);
                            return subjectDn;
                        })
                )
                .authorizeExchange(exchanges -> exchanges
                        .pathMatchers("/api/payments/health").permitAll()
                        .anyExchange().authenticated()
                )
                .build();
    }

    private String extractSpiffeId(X509Certificate cert) {
        try {
            if (cert.getSubjectAlternativeNames() == null) return null;

            for (List<?> san : cert.getSubjectAlternativeNames()) {
                if (san.size() == 2 && (Integer) san.get(0) == 6) {
                    return (String) san.get(1);
                }
            }
        } catch (Exception e) {
            log.warn("Failed to parse SAN from certificate", e);
        }
        return null;
    }

    @Bean
    public ReactiveUserDetailsService userDetailsService() {
        return username -> {
            return Mono.just(
                    new User(
                            username,
                            "",
                            Collections.singletonList(new SimpleGrantedAuthority("ROLE_SPIFFE_CLIENT"))
                    )
            );
        };
    }
}