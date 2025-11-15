package com.example.ordersservice.dto;

public class PaymentRequest {
    private String orderId;
    private double amount;

    public PaymentRequest(String orderId, double amount) {
        this.orderId = orderId;
        this.amount = amount;
    }

    // Getters and setters
    public String getOrderId() {
        return orderId;
    }

    public void setOrderId(String orderId) {
        this.orderId = orderId;
    }

    public double getAmount() {
        return amount;
    }

    public void setAmount(double amount) {
        this.amount = amount;
    }
}
