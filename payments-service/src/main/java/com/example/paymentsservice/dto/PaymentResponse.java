package com.example.paymentsservice.dto;

public record PaymentResponse(
        String transactionId,
        String status,
        String message
) {}
