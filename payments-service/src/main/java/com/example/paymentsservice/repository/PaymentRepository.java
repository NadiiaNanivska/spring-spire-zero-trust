package com.example.paymentsservice.repository;

import com.example.paymentsservice.entity.Payment;
import org.springframework.stereotype.Repository;
import reactor.core.publisher.Mono;

import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Repository
public class PaymentRepository {

    // Наша "База даних" в оперативній пам'яті
    // ConcurrentHashMap гарантує безпеку при доступі з багатьох потоків Netty
    private final Map<String, Payment> storage = new ConcurrentHashMap<>();

    // Зберігання даних
    public Mono<Payment> save(Payment payment) {
        return Mono.fromSupplier(() -> {
            storage.put(payment.transactionId(), payment);
            return payment;
        });
    }

    // Пошук (на випадок, якщо захочеш зробити endpoint GET /status/{id})
    public Mono<Payment> findById(String transactionId) {
        return Mono.justOrEmpty(storage.get(transactionId));
    }

    // Метод для метрик - подивитися скільки всього записів
    public int count() {
        return storage.size();
    }
}