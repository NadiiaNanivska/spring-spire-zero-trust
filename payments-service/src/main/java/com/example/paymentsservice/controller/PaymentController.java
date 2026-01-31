package com.example.paymentsservice.controller;

import com.example.paymentsservice.dto.PaymentRequest;
import com.example.paymentsservice.dto.PaymentResponse;
import com.example.paymentsservice.service.PaymentService;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.server.ServerWebExchange;
import reactor.core.publisher.Mono;

@Slf4j
@RequiredArgsConstructor
@RestController
@RequestMapping("/api/payments")
public class PaymentController {

    private final PaymentService paymentService;

    @PostMapping("/process")
    public Mono<PaymentResponse> process(
            @RequestBody PaymentRequest request,
            ServerWebExchange exchange
    ) {
        return exchange.getPrincipal()
                .flatMap(principal -> {
                    log.info("Secure Request authenticated via mTLS. Caller Identity: {}", principal.getName());

                    return paymentService.processPayment(request);
                })
                .switchIfEmpty(Mono.defer(() -> {
                    log.warn("Request without Identity!");
                    return paymentService.processPayment(request);
                }));
    }
}
