# Database Persistence Layer - Quick Overview

## What Was Done

Added PostgreSQL database persistence to store all order data in the microservices-demo application.

## Quick Deploy

```bash
kubectl apply -f kubernetes-manifests/postgres.yaml
kubectl apply -f kubernetes-manifests/
```

## Files Created

1. **kubernetes-manifests/postgres.yaml** - PostgreSQL deployment
2. **src/checkoutservice/database.go** - Database access layer
3. **DATABASE_DEPLOYMENT.md** - Full deployment guide
4. **QUICK_START_DATABASE.md** - Quick reference
5. **CHANGES_SUMMARY.md** - Technical details

## Files Modified

1. **src/checkoutservice/main.go** - Added database integration
2. **src/checkoutservice/go.mod** - Added PostgreSQL driver
3. **kubernetes-manifests/checkoutservice.yaml** - Added DATABASE_URL

## Database Schema

- **orders table**: Stores order details (order_id, user info, amounts, addresses)
- **order_items table**: Stores line items (product_id, quantity, cost)

## Verify Deployment

```bash
kubectl logs -l app=checkoutservice | grep database
kubectl exec -it deployment/postgres -- psql -U orderservice -d ordersdb -c "SELECT COUNT(*) FROM orders;"
```

## Documentation

- [DATABASE_DEPLOYMENT.md](./DATABASE_DEPLOYMENT.md) - Complete guide
- [QUICK_START_DATABASE.md](./QUICK_START_DATABASE.md) - Quick reference
- [CHANGES_SUMMARY.md](./CHANGES_SUMMARY.md) - Technical details
