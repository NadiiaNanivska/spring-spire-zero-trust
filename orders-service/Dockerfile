FROM eclipse-temurin:17-alpine

WORKDIR /app

COPY target/orders-service-0.0.1-SNAPSHOT.jar orders-service.jar

ENTRYPOINT ["java", "-jar", "orders-service.jar"]
