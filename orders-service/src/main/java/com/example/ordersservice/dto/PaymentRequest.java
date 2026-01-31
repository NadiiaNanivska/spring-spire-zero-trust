package com.example.ordersservice.dto;

public record PaymentRequest(String orderId, double amount, String concurrency) {
}
