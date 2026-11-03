FROM eclipse-temurin:17-alpine

WORKDIR /app

COPY target/orders-service-0.0.1-SNAPSHOT.jar orders-service.jar
COPY target/orders-service-0.0.1-SNAPSHOT.jar.sig orders-service.jar.sig

ENTRYPOINT ["java", "-jar", "orders-service.jar"]
