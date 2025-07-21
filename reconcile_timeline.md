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
                │ ├─ Add finalizer (r.Update) ←─ QUEUES RECONCILE 2 (but doesn't start yet)                 │
                │ ├─ === FINALIZER ADDED - This update will trigger a NEW reconcile loop ===                │
                │ ├─ === CONTINUING WITH EC2 INSTANCE CREATION IN CURRENT RECONCILE ===                     │
                │ ├─ createEc2Instance()                                                                    │
                │ │  ├─ === STARTING EC2 INSTANCE CREATION ===                                              │
                │ │  ├─ === CALLING AWS RunInstances API ===                                                │
                │ │  ├─ === EC2 INSTANCE CREATED SUCCESSFULLY === (Instance ID available)                   │
                │ │  ├─ === WAITING 10 SECONDS FOR INSTANCE TO INITIALIZE === ──────────────────────────┐   │
                │ │  │                                                                                  │   │
                │ │  │  [████████████████████████ 10 SECOND WAIT ████████████████████████]              │   │
                │ │  │                                                                                  │   │
                │ │  ├─ === CALLING AWS DescribeInstances API TO GET INSTANCE DETAILS === <─────────────┘   │
                │ │  └─ === EC2 INSTANCE CREATION COMPLETED ===                                             │
                │ ├─ === ABOUT TO UPDATE STATUS - This will trigger reconcile loop again ===                │
                │ ├─ Update status (r.Status().Update) ←─ QUEUES RECONCILE 3                                │
                │ ├─ === STATUS UPDATED - Reconcile loop will be triggered again ===                        │
                │ └─ Return success                                                                         │
                └───────────────────────────────────────────────────────────────────────────────────────────┘

Reconcile 2:                                                                                                  ┌───────────────┐
(Triggered by                                                                                                 │ === RECONCILE LOOP STARTED === │
 finalizer update)                                                                                            │ ├─ Get resource                │
                                                                                                              │ ├─ Check Status.InstanceID     │
                                                                                                              │ │  (NOT EMPTY - has ID!)       │
                                                                                                              │ ├─ "Instance already exists"   │
                                                                                                              │ ├─ checkEC2InstanceExists()    │
                                                                                                              │ └─ Return success              │
                                                                                                              └───────────────┘

Reconcile 3:                                                                                                                    ┌─────┐
(Triggered by                                                                                                                     │ === RECONCILE LOOP STARTED === │
 status update)                                                                                                                   │ ├─ Get resource                │
                                                                                                                                  │ ├─ Check Status.InstanceID     │
                                                                                                                                  │ │  (has ID)                    │
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
2. **Finalizer Addition**: `r.Update()` is called to add the finalizer BEFORE creating the EC2 instance - this QUEUES Reconcile 2 but doesn't interrupt current execution
3. **Instance Creation**: The `createEc2Instance()` function includes a 10-second `time.Sleep()` to wait for the instance to initialize
4. **Status Update**: After instance creation, `r.Status().Update()` is called to update the resource status with the instance ID
5. **Reconcile 2** is triggered by the finalizer update and DOES see the populated InstanceID because it runs AFTER Reconcile 1 completes
6. **Reconcile 3** is triggered by the status update and also sees the populated InstanceID

## Important Behavior:

**Controller-runtime queues reconcile requests but processes them sequentially:**

- When `r.Update()` or `r.Status().Update()` is called, it registers a new reconcile request in the queue
- The new reconcile DOES NOT start immediately - it waits for the current reconcile to complete
- This is why Reconcile 2 sees the instance ID - it starts after Reconcile 1 has finished all its work, including the status update

## Code Flow with Log Messages:

```go
// Reconcile 1 (runs for ~11+ seconds)
"=== RECONCILE LOOP STARTED ==="
"Creating new instance"
"=== ABOUT TO ADD FINALIZER ==="
r.Update() // Queues Reconcile 2 but doesn't start it
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
r.Status().Update() // Queues Reconcile 3
"=== STATUS UPDATED - Reconcile loop will be triggered again ==="
// Returns - NOW queued reconciles can start

// Reconcile 2 (starts AFTER Reconcile 1 completes)
"=== RECONCILE LOOP STARTED ==="
"Instance already exists" // Sees the instance ID!

// Reconcile 3 (starts after Reconcile 2 completes)
"=== RECONCILE LOOP STARTED ==="
"Instance already exists"
```

## Important Implementation Details:

- The controller checks if `ec2Instance.Status.InstanceID != ""` to determine if an instance already exists
- Reconcile requests are queued and processed one at a time for the same resource
- The 10-second wait happens inside the `createEc2Instance()` function to allow AWS to populate public IP/DNS
- Both Reconcile 2 and 3 see the instance ID because they run after Reconcile 1 has completed all its work
