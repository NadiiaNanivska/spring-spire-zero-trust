package com.example.ordersservice.controller;

import com.example.ordersservice.dto.OrderRequest;
import com.example.ordersservice.dto.PaymentRequest;
import com.example.ordersservice.dto.PaymentResponse;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.UUID;

@Slf4j
@RestController
@RequestMapping("/api/orders")
@RequiredArgsConstructor
public class OrderController {

    private final WebClient webClient;

    @Value("${app.services.payments-url}")
    private String paymentsServiceUrl;

    @PostMapping("/create")
    public Mono<PaymentResponse> createOrder(@RequestBody OrderRequest orderRequest) {
        log.info("Initiating order for item: {}", orderRequest.itemId());

        PaymentRequest paymentRequest = new PaymentRequest(
                UUID.randomUUID().toString(),
                orderRequest.quantity(),
                "USD"
        );

        return webClient.post()
                .uri(paymentsServiceUrl + "/api/payments/process")
                .bodyValue(paymentRequest)
                .retrieve()
                .bodyToMono(PaymentResponse.class)
                .doOnSuccess(res -> log.info("Payment successful: {}", res.transactionId()))
                .doOnError(err -> log.error("Payment failed: {}", err.getMessage()));
    }
}