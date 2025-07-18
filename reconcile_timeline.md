# Reconcile Loop Timeline Diagram

## Timeline of Reconcile Loop Execution

```
Time →    0ms    10ms   20ms   30ms   40ms   50ms   60ms   70ms   80ms   90ms   100ms  110ms  120ms
         │       │      │      │      │      │      │      │      │      │      │      │      │
         ▼       ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼
Reconcile 1:    ┌─────────────────────────────────────────────────────────────────────────────┐
                │ RECONCILE LOOP STARTED                                                     │
                │ ├─ Get resource                                                             │
                │ ├─ Check if instance exists (no)                                           │
                │ ├─ Add finalizer (r.Update) ←─ NEW RECONCILE 2 REGISTERED HERE              │
                │ ├─ Continue execution                                                       │
                │ ├─ Create EC2 instance (10s wait)                                          │
                │ ├─ Update status (r.Status().Update) ←─ NEW RECONCILE 3 REGISTERED HERE     │
                │ └─ Return success                                                           │
                └─────────────────────────────────────────────────────────────────────────────┘

Reconcile 2:                                                                  ┌───────────────┐
                                                                              │ RECONCILE LOOP STARTED │
                                                                              │ ├─ Get resource        │
                                                                              │ ├─ Check if instance   │
                                                                              │ │  exists (yes)        │
                                                                              │ └─ Return success      │
                                                                              └───────────────┘

Reconcile 3:                                                                                    ┌─────┐
                                                                                                  │ RECONCILE LOOP STARTED │
                                                                                                  │ ├─ Get resource        │
                                                                                                  │ ├─ Check if instance   │
                                                                                                  │ │  exists (yes)        │
                                                                                                  │ └─ Return success      │
                                                                                                  └─────┘

K8s Watch:      ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐
                │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│
                │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│
                └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘
                Finalizer Status  Status   Status   Status   Status   Status   Status   Status
                Added    Update   Update   Update   Update   Update   Update   Update   Update
```

## Key Points:

1. **Reconcile 1** starts immediately when resource is created
2. **r.Update()** (finalizer) registers Reconcile 2 but doesn't interrupt Reconcile 1
3. **r.Status().Update()** registers Reconcile 3 but doesn't interrupt Reconcile 1
4. **Reconcile 2** starts after Reconcile 1 completes (around 60ms)
5. **Reconcile 3** starts after Reconcile 2 completes (around 70ms)
6. **Kubernetes Watch** continuously monitors for changes and triggers new reconciles

## Registration vs Execution:

- **Registration**: Happens immediately when `r.Update()` or `r.Status().Update()` is called
- **Execution**: Happens after the current reconcile completes, not immediately
- **Queue**: Kubernetes controller-runtime maintains a queue of reconcile requests
- **Order**: Reconciles execute in FIFO (First In, First Out) order

## Why This Matters:

- The current reconcile function continues executing even after triggering new reconciles
- This prevents infinite loops and ensures predictable behavior
- Each reconcile sees the latest state of the resource
- The finalizer ensures cleanup happens even if the controller crashes
