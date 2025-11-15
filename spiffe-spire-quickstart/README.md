# SPIFFE & SPIRE Deployment for Kubernetes

This repository provides Kubernetes manifests to deploy **SPIFFE** and **SPIRE** in your cluster.  
It includes configuration for both the **SPIRE Server** and **SPIRE Agent**, allowing workloads to securely obtain and use SPIFFE identities (SVIDs) for authentication and authorization.

---

## 📂 Repository Contents

- **spire-namespace.yaml**  
  Creates the dedicated `spire` namespace.  

- **spire-bundle-configmap.yaml**  
  Provides the trust bundle (root CA certificates) for SPIRE.  

- **server-account.yaml**  
  Defines the ServiceAccount used by the SPIRE Server.  

- **server-cluster-role.yaml**  
  Grants the necessary RBAC permissions to the SPIRE Server.  

- **server-configmap.yaml**  
  Configures the SPIRE Server.  

- **server-service.yaml**  
  Exposes the SPIRE Server inside the cluster.  

- **server-statefulset.yaml**  
  Deploys the SPIRE Server with persistence and stable identity.  

- **agent-account.yaml**  
  Defines the ServiceAccount for SPIRE Agents.  

- **agent-cluster-role.yaml**  
  Grants RBAC permissions required by SPIRE Agents.  

- **agent-configmap.yaml**  
  Provides configuration for SPIRE Agents.  

- **agent-daemonset.yaml**  
  Ensures a SPIRE Agent runs on each Kubernetes node.  


---


