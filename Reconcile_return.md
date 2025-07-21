```go
// 1. Success - done, wait for next event
return ctrl.Result{}, nil                    // Same as {Requeue: false}, nil

// 2. Error - let Kubernetes retry with backoff
return ctrl.Result{}, err                    

// 3. Need to check again soon
return ctrl.Result{RequeueAfter: 30 * time.Second}, nil

// 4. Immediate requeue (use sparingly!)
return ctrl.Result{Requeue: true}, nil      

// 5. These are redundant/confusing - avoid
return ctrl.Result{Requeue: false}, nil     // Just use {}, nil
return ctrl.Result{Requeue: true}, err      // Just use {}, err
```
