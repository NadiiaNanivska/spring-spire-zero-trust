FROM eclipse-temurin:17-alpine

WORKDIR /app

COPY target/payments-service-0.0.1-SNAPSHOT.jar payments-service.jar

ENTRYPOINT ["java", "-jar", "payments-service.jar"]
