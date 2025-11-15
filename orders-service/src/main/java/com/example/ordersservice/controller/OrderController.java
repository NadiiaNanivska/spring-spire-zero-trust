package com.example.ordersservice.controller;

import com.example.ordersservice.dto.OrderRequest;
import com.example.ordersservice.dto.PaymentRequest;
import com.example.ordersservice.dto.PaymentResponse;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.reactive.function.client.WebClient;
import reactor.core.publisher.Mono;

import java.util.UUID;

@RestController
@RequestMapping("/api/orders")
public class OrderController {

    private final WebClient webClient;

    public OrderController(WebClient webClient) {
        this.webClient = webClient;
    }

    @PostMapping("/create")
    public Mono<PaymentResponse> createOrder(@RequestBody OrderRequest orderRequest) {
        PaymentRequest paymentRequest = new PaymentRequest(UUID.randomUUID().toString(), 100.0); // Dummy amount
        return webClient.post()
                .uri("http://localhost:8081/api/payments/process")
                .bodyValue(paymentRequest)
                .retrieve()
                .bodyToMono(PaymentResponse.class);
    }
}
