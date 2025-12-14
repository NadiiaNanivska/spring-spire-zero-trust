package com.example.paymentsservice.dto;

public record PaymentRequest(
        String orderId,
        Double amount,
        String currency
) {}
