package com.example.paymentsservice.controller;

import com.example.paymentsservice.dto.PaymentRequest;
import com.example.paymentsservice.dto.PaymentResponse;
import com.example.paymentsservice.service.PaymentService;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;
import reactor.core.publisher.Mono;

import java.util.UUID;

@Slf4j
@RequiredArgsConstructor
@RestController
@RequestMapping("/api/payments")
public class PaymentController {

    private final PaymentService paymentService;

    @PostMapping("/process")
    public Mono<PaymentResponse> process(@RequestBody PaymentRequest request) {
        // Тут немає перевірки паролів!
        // Автентифікація відбулася на рівні TLS (SslContext),
        // а Авторизація (перевірка SPIFFE ID) буде в Security Filter Chain (окремий клас)
        return paymentService.processPayment(request);
    }

    @GetMapping("/health")
    public Mono<String> health() {
        return Mono.just("Payments Service is Running");
    }
}
