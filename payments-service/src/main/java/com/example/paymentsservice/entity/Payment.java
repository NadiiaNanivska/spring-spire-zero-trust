package com.example.paymentsservice.entity;

import java.time.Instant;

public record Payment(
        String transactionId,
        String orderId,
        Double amount,
        String status,
        Instant timestamp
) {}
