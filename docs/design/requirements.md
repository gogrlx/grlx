1 Million endpoints
2. DMZ and non DMZ separation
3. Windows and Unix support
4. Deployment automation using ansible 
5. API key based authentication to NATS websocket
6. Each sprout has a JWT Token to connect to the URL
7. Farmer is horizontally scalable 
8. Sprout supports connections via proxies
9. Sprout can download recipes via a dedicated http endpoint which is configured in sprout
10. NATS response to sprout less than 300 ms
11. Recipe download should authenticate with the same key 
12. We will use envoy with JWT token validation in front of NATS
13. We will run the backend on Kubernetes for NATS, Valkey, farmer and Percona
14. Payload in NATS is encrypted using key pair per sprout and keypair per tenant in master
15. Key rotation feature needed for sprout private and public keys. A new transaction which will send the private key encrypted in NATS
16. SDB support equivalent to salt in the sprout. Sprout can use secrets from external sources as per recipe definition
17. include probe capability in sprout to support database and http sequences. Run probe sequence would be a sprout task
18. Installers for yum, apt, zipper and windows package (msi)
19. Ansible playbook to use installer to deploy sprout with one time key to download JWT token and keys. 

