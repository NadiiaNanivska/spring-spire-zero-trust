package com.example.paymentsservice.service;

import com.example.paymentsservice.dto.PaymentRequest;
import com.example.paymentsservice.repository.PaymentRepository;
import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.Test;
import reactor.core.publisher.Mono;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

class PaymentServiceTest {
    @Test
    void countsOnlyCompletedPayments() {
        SimpleMeterRegistry registry = new SimpleMeterRegistry();
        PaymentRepository repository = mock(PaymentRepository.class);
        when(repository.save(any())).thenAnswer(invocation -> Mono.just(invocation.getArgument(0)));
        PaymentService service = new PaymentService(repository, registry);
        PaymentRequest request = new PaymentRequest("order-1", 10.0, "USD");

        var pending = service.processPayment(request);
        var counter = registry.get("payments.processed").tag("outcome", "success").counter();
        assertEquals(0.0, counter.count());
        assertEquals("CONFIRMED", pending.block(Duration.ofSeconds(5)).status());
        assertEquals(1.0, counter.count());

        when(repository.save(any())).thenReturn(Mono.error(new IllegalStateException("save failed")));
        assertThrows(IllegalStateException.class,
                () -> service.processPayment(request).block(Duration.ofSeconds(5)));
        assertEquals(1.0, counter.count());
    }
}
