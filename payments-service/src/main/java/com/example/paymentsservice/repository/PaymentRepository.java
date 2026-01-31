package com.example.paymentsservice.repository;

import com.example.paymentsservice.entity.Payment;
import org.springframework.stereotype.Repository;
import reactor.core.publisher.Mono;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Repository
public class PaymentRepository {

    private final Map<String, Payment> storage = new ConcurrentHashMap<>();

    public Mono<Payment> save(Payment payment) {
        return Mono.fromSupplier(() -> {
            storage.put(payment.transactionId(), payment);
            return payment;
        });
    }

    public int count() {
        return storage.size();
    }
}