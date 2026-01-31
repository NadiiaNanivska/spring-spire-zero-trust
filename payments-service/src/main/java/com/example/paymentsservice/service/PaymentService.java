package com.example.paymentsservice.service;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import reactor.core.publisher.Mono;

import com.example.paymentsservice.dto.PaymentRequest;
import com.example.paymentsservice.dto.PaymentResponse;
import com.example.paymentsservice.entity.Payment;
import com.example.paymentsservice.repository.PaymentRepository;

import java.time.Duration;
import java.time.Instant;
import java.util.UUID;

@Slf4j
@Service
@RequiredArgsConstructor
public class PaymentService {

    private final PaymentRepository repository;

    public Mono<PaymentResponse> processPayment(PaymentRequest request) {
        log.info("Received payment request for Order ID: {}", request.orderId());

        String txId = UUID.randomUUID().toString();
        Payment entity = new Payment(
                txId,
                request.orderId(),
                request.amount(),
                "SUCCESS",
                Instant.now()
        );

        return Mono.just(entity)
                .delayElement(Duration.ofMillis(20))
                .flatMap(repository::save)
                .map(savedPayment -> {
                    log.info("Payment processed successfully. TxID: {}", savedPayment.transactionId());
                    return new PaymentResponse(
                            savedPayment.transactionId(),
                            "CONFIRMED",
                            "Processed securely via SPIRE mTLS"
                    );
                });
    }
}