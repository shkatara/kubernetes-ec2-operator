# Reconcile Loop Timeline Diagram

## Timeline of Reconcile Loop Execution

```
Time →    0s     1s     2s     3s     4s     5s     6s     7s     8s     9s     10s    11s    12s    13s
         │       │      │      │      │      │      │      │      │      │      │      │      │      │
         ▼       ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼      ▼
Reconcile 1:    ┌───────────────────────────────────────────────────────────────────────────────────────────┐
                │ === RECONCILE LOOP STARTED ===                                                            │
                │ ├─ Get resource                                                                           │
                │ ├─ Check if instance exists (Status.InstanceID == "")                                     │
                │ ├─ === ABOUT TO ADD FINALIZER ===                                                         │
                │ ├─ Add finalizer (r.Update) ←─ NEW RECONCILE 2 REGISTERED HERE                            │
                │ ├─ === FINALIZER ADDED - This update will trigger a NEW reconcile loop ===                │
                │ ├─ === CONTINUING WITH EC2 INSTANCE CREATION IN CURRENT RECONCILE ===                     │
                │ ├─ createEc2Instance()                                                                    │
                │ │  ├─ === STARTING EC2 INSTANCE CREATION ===                                              │
                │ │  ├─ === CALLING AWS RunInstances API ===                                                │
                │ │  ├─ === EC2 INSTANCE CREATED SUCCESSFULLY ===                                           │
                │ │  ├─ === WAITING 10 SECONDS FOR INSTANCE TO INITIALIZE === ──────────────────────────┐   │
                │ │  │                                                                                  │   │
                │ │  │  [████████████████████████ 10 SECOND WAIT ████████████████████████]              │   │
                │ │  │                                                                                  │   │
                │ │  ├─ === CALLING AWS DescribeInstances API TO GET INSTANCE DETAILS === <─────────────┘   │
                │ │  └─ === EC2 INSTANCE CREATION COMPLETED ===                                             │
                │ ├─ === ABOUT TO UPDATE STATUS - This will trigger reconcile loop again ===                │
                │ ├─ Update status (r.Status().Update) ←─ NEW RECONCILE 3 REGISTERED HERE                   │
                │ ├─ === STATUS UPDATED - Reconcile loop will be triggered again ===                        │
                │ └─ Return success                                                                         │
                └───────────────────────────────────────────────────────────────────────────────────────────┘

Reconcile 2:                                                                                              ┌───────────────┐
(Triggered by                                                                                             │ === RECONCILE LOOP STARTED === │
 finalizer update)                                                                                        │ ├─ Get resource                │
                                                                                                          │ ├─ Check Status.InstanceID     │
                                                                                                          │ │  (still empty, creation      │
                                                                                                          │ │   in progress)               │
                                                                                                          │ └─ Return success              │
                                                                                                          └───────────────┘

Reconcile 3:                                                                                                                ┌─────┐
(Triggered by                                                                                                                 │ === RECONCILE LOOP STARTED === │
 status update)                                                                                                               │ ├─ Get resource                │
                                                                                                                              │ ├─ Check Status.InstanceID     │
                                                                                                                              │ │  (now populated)             │
                                                                                                                              │ ├─ "Instance already exists"   │
                                                                                                                              │ ├─ checkEC2InstanceExists()    │
                                                                                                                              │ └─ Return success              │
                                                                                                                              └─────┘

K8s Watch Events: ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐
                  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│  │Watch│
                  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│  │Event│
                  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘  └─────┘
                  Resource Finalizer                                                                          Status
                  Created  Added                                                                              Updated
```

## Key Points from Actual Code:

1. **Reconcile 1** starts when the EC2Instance custom resource is created
2. **Finalizer Addition**: `r.Update()` is called to add the finalizer BEFORE creating the EC2 instance
3. **Instance Creation**: The `createEc2Instance()` function includes a 10-second `time.Sleep()` to wait for the instance to initialize
4. **Status Update**: After instance creation, `r.Status().Update()` is called to update the resource status
5. **Reconcile 2** is triggered by the finalizer update but likely sees an empty InstanceID (creation still in progress)
6. **Reconcile 3** is triggered by the status update and sees the populated InstanceID

## Code Flow with Log Messages:

```go
// Reconcile 1
"=== RECONCILE LOOP STARTED ==="
"Creating new instance"
"=== ABOUT TO ADD FINALIZER ==="
r.Update() // Triggers Reconcile 2
"=== FINALIZER ADDED - This update will trigger a NEW reconcile loop, but current reconcile continues ==="
"=== CONTINUING WITH EC2 INSTANCE CREATION IN CURRENT RECONCILE ==="
  createEc2Instance():
    "=== STARTING EC2 INSTANCE CREATION ==="
    "=== CALLING AWS RunInstances API ==="
    "=== EC2 INSTANCE CREATED SUCCESSFULLY ==="
    "=== WAITING 10 SECONDS FOR INSTANCE TO INITIALIZE ==="
    time.Sleep(10 * time.Second)
    "=== CALLING AWS DescribeInstances API TO GET INSTANCE DETAILS ==="
    "=== EC2 INSTANCE CREATION COMPLETED ==="
"=== ABOUT TO UPDATE STATUS - This will trigger reconcile loop again ==="
r.Status().Update() // Triggers Reconcile 3
"=== STATUS UPDATED - Reconcile loop will be triggered again ==="

// Reconcile 2 (while Reconcile 1 is still running)
"=== RECONCILE LOOP STARTED ==="
"Instance already exists" or continues creation flow

// Reconcile 3 (after Reconcile 1 completes)
"=== RECONCILE LOOP STARTED ==="
"Instance already exists"
```

## Important Implementation Details:

- The controller checks if `ec2Instance.Status.InstanceID != ""` to determine if an instance already exists
- If the instance doesn't exist, it adds a finalizer and creates the instance in the same reconcile loop
- The 10-second wait happens inside the `createEc2Instance()` function to allow AWS to populate public IP/DNS
- The controller uses structured logging with clear markers for each phase
- Deletion is handled by checking `DeletionTimestamp.IsZero()` and removing the finalizer after deletion
