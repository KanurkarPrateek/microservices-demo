# Quick Start - Database Deployment

Fast deployment guide for adding PostgreSQL persistence to microservices-demo.

## One-Command Deploy (Local Development)

```bash
# Navigate to the project root
cd microservices-demo

# Deploy everything including database
kubectl apply -f kubernetes-manifests/postgres.yaml
kubectl apply -f kubernetes-manifests/

# Wait for all pods to be ready
kubectl wait --for=condition=ready pod --all --timeout=300s
```

## Verify Deployment

```bash
# Check all pods are running
kubectl get pods

# Check database connection in checkout service
kubectl logs -l app=checkoutservice | grep -i database

# Test by placing an order through the frontend, then:
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "SELECT COUNT(*) FROM orders;"
```

## What Was Added

### New Files
- `kubernetes-manifests/postgres.yaml` - PostgreSQL deployment
- `src/checkoutservice/database.go` - Database access layer
- `DATABASE_DEPLOYMENT.md` - Full documentation

### Modified Files
- `src/checkoutservice/main.go` - Added database integration
- `src/checkoutservice/go.mod` - Added PostgreSQL driver
- `kubernetes-manifests/checkoutservice.yaml` - Added DATABASE_URL env var

## Database Schema

```sql
-- Orders table
orders (
    order_id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255),
    user_email VARCHAR(255),
    total_amount_units BIGINT,
    total_amount_nanos INTEGER,
    shipping_tracking_id VARCHAR(255),
    created_at TIMESTAMP
)

-- Order items table
order_items (
    id SERIAL PRIMARY KEY,
    order_id VARCHAR(255) REFERENCES orders(order_id),
    product_id VARCHAR(255),
    quantity INTEGER,
    cost_units BIGINT,
    cost_nanos INTEGER
)
```

## Default Configuration

- Database: `ordersdb`
- User: `orderservice`
- Password: `orderpass123`
- Host: `postgres:5432`
- Storage: 5Gi PersistentVolume

**⚠️ Change credentials for production!**

## Common Commands

```bash
# Connect to database
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb

# View recent orders
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c \
  "SELECT order_id, user_id, created_at FROM orders ORDER BY created_at DESC LIMIT 10;"

# Count total orders
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c \
  "SELECT COUNT(*) as total_orders FROM orders;"

# View order details
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c \
  "SELECT o.order_id, o.user_email, oi.product_id, oi.quantity 
   FROM orders o 
   JOIN order_items oi ON o.order_id = oi.order_id 
   LIMIT 5;"

# Backup database
kubectl exec deployment/postgres -- pg_dump -U orderservice ordersdb > backup.sql

# Restore database
kubectl exec -i deployment/postgres -- psql -U orderservice ordersdb < backup.sql
```

## Rebuild Checkout Service (If Needed)

```bash
cd src/checkoutservice

# Build and push (replace with your registry)
docker build -t your-registry/checkoutservice:latest .
docker push your-registry/checkoutservice:latest

# Update deployment
kubectl set image deployment/checkoutservice server=your-registry/checkoutservice:latest
```

## Troubleshooting

**Database not connecting?**
```bash
# Check postgres pod status
kubectl get pods -l app=postgres

# Check postgres logs
kubectl logs -l app=postgres

# Check checkout service can resolve postgres
kubectl exec -it deployment/checkoutservice -- nslookup postgres
```

**Orders not saving?**
```bash
# Check checkout service logs for errors
kubectl logs -l app=checkoutservice | grep -i error

# Verify DATABASE_URL is set
kubectl describe deployment checkoutservice | grep DATABASE_URL
```

**Need to reset database?**
```bash
# Delete all data
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "TRUNCATE orders CASCADE;"

# Or delete and recreate
kubectl delete -f kubernetes-manifests/postgres.yaml
kubectl delete pvc postgres-pv-claim
kubectl apply -f kubernetes-manifests/postgres.yaml
```

## Architecture

```
┌─────────────┐
│  Frontend   │
│  (Browser)  │
└──────┬──────┘
       │
       ▼
┌─────────────────┐
│   Checkout      │
│   Service       │◄──────┐
│   (Go + DB)     │       │
└─────────┬───────┘       │
          │               │
          │  Saves Orders │
          ▼               │
    ┌──────────┐          │
    │PostgreSQL│──────────┘
    │ Database │    Reads Orders
    └──────────┘
         │
         ▼
  [Persistent Storage]
```

## Next Steps

- Review [DATABASE_DEPLOYMENT.md](./DATABASE_DEPLOYMENT.md) for production setup
- Configure Kubernetes Secrets for credentials
- Set up automated backups
- Enable monitoring and alerting
- Consider managed database for production (Cloud SQL, RDS, etc.)

## Security Note

Default setup uses plaintext passwords in ConfigMaps. For production:

1. Create a Kubernetes Secret:
```bash
kubectl create secret generic db-credentials \
  --from-literal=url="postgres://user:pass@postgres:5432/ordersdb?sslmode=require"
```

2. Update checkoutservice.yaml:
```yaml
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: db-credentials
      key: url
```

3. Enable SSL in postgres.yaml and update connection string
