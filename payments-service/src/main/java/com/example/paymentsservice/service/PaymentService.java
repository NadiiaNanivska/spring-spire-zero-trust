package com.example.paymentsservice.service;

import io.micrometer.core.instrument.Counter;
import io.micrometer.core.instrument.MeterRegistry;
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
public class PaymentService {

    private final PaymentRepository repository;
    private final Counter successfulPayments;

    public PaymentService(PaymentRepository repository, MeterRegistry meterRegistry) {
        this.repository = repository;
        this.successfulPayments = Counter.builder("payments.processed")
                .description("Number of successfully processed payments")
                .tag("outcome", "success")
                .register(meterRegistry);
    }

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
                    successfulPayments.increment();
                    log.info("Payment processed successfully. TxID: {}", savedPayment.transactionId());
                    return new PaymentResponse(
                            savedPayment.transactionId(),
                            "CONFIRMED",
                            "Processed securely via SPIRE mTLS"
                    );
                });
    }
}