FROM eclipse-temurin:17-alpine

WORKDIR /app

COPY target/payments-service-0.0.1-SNAPSHOT.jar payments-service.jar
COPY target/payments-service-0.0.1-SNAPSHOT.jar.sig payments-service.jar.sig

ENTRYPOINT ["java", "-jar", "payments-service.jar"]
