# Stklog Hook for [Logrus](https://github.com/Sirupsen/logrus)

Use this hook to send your logs to [Stklog.io](https://stklog.io) server over HTTP. The hook is non-blocking and send batch of logs to Stklog via a goroutine.

All `logrus` fields will be sent as extra fields on `Stklog.io`

## Usage

The hook must be configured with:

* Your project Project key
* Love

```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func main() {
    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    // instantiate a "stack" where your logs will be linked to
    stklog.CreateStack().End()
    log.AddHook(hook)
    log.Info("some logging message")
}
```

***NB: hook.Flush is used to flush the buffer of logs/stacks before quitting, useful if you're not a daemon running forever or to quite properly.***
### Custom level of logging for Stklog.io

You can define a level of logging for `Stklog.io` independently from `logrus` itself.

```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func main() {
    log.SetOutput(os.Stdout)
    // logrus level = info
    log.SetLevel(log.InfoLevel)

    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    stklog.CreateStack().End()
    // stklog level = warn
    hook.SetLevel(log.WarnLevel)
    log.AddHook(hook)
    // this will be log on stdout but not into stklog.io
    log.Info("some logging message")
}
```
## Logs
Logs written by Logrus, will be standardized to be accepted by `Stklog.io` API.
Here are the current version of the struct used :

**Level**  
>`Debug`, `Info`, `Warning`, `Error`, `Fatal` and  `Panic`

> For a better insight in Stklog.io we advise to use at least `Info` levels

**Timestamp**
> Using [RFC3339](https://golang.org/pkg/time/#pkg-constants)

**File**
> In which file the log call was performed

**Line**
> On which line of this file it was.

**RequestID**
> Unique identifier letting us link this log to a `stack`

**Message**
> Well, the log itself

**Extra**
> Extra fields from logrus


## Stacks
`Stklog.io` as a different way to deal with logs, instead of sending your logs all independently from each others, and then visualize a soup of mixed logs coming from different services, threads and scopes. They introduce a concept of stack, like you would imagine a stacktrace. Because your logs are actually events triggered in a logical order. A stack then, represent a block of logs ordered by time. Moreover, a stack can be attached to another one, to produce a sub stack, like a sub logical part of your current context. Really useful for microservices or threads/goroutines tracking ..

### End
You will need to call End() for the stack to be created on `Stklog.io`. Stacks and logs will be bufferised and sent over HTTPS to `Stklog.io` every 15 seconds asynchronously to avoid having any impact to your service.

### Attach
Since we consider a stack as a logical block of events, it makes sense to allow you to create sub blocks or child stacks that are linked with their parents.

A really simple case in Golang would be the creation of a goroutine, being part of the same tree of logic, but representing a new sub tree by itself.

Attach return a new stack and a potential error. One common cause of error, is that, trying to attach a stack that as not been ended (called End()), won't let you attach it to a child.


```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func worker(stack *stklog.Stack) {
    // childStack is linked to your parent stack
    childStack, err := stack.Attach()
    if err == nil {
        childStack.SetName("main.worker").End()
    }
    // any logs after that, inside this goroutine, will be linked to childStack and not its parent.
    log.Info("test inside main.worker")
}

func main() {
    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    stack := stklog.CreateStack().SetName("main").End()
    log.AddHook(hook)
    log.Info("Starting main loop")
    go worker(stack)
    for {
      // main loop doing something
    }
}
```
#### Edge case
If you call `Attach` in the same thread (goroutine or not).
Then it will map again your current thread with the newly created stack and you will lose the possibility to write logs to the parent.
That's why, we advise to call `Attach` only inside a goroutine ...

However, if you still need to attach a new stack to the parent in the same thread and then want to switch back to the parent stack. You can manage on your side the unique ID of each stack, we explain this possibility later.  

### New independant stack
You can as well, not link your stack with each others when using goroutines. Or you could decide to stop a stack at some point and create a new one in the same thread.
You just have to call `CreateStack` again and it will link any new log of the current thread to this new stack.

#### Example with a goroutine
```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func worker() {
    stklog.CreateStack().SetName("worker").End()
    log.Info("logs inside worker, independently of main")
}

func main() {
    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    stklog.CreateStack().SetName("main").End()
    log.AddHook(hook)
    log.Info("Starting main loop")
    go worker()
    for {
      // main loop doing something
    }
}
```

#### Example in the same thread
```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func worker(stack *stklog.Stack) {
    log.Info("logs written to the 'worker' stack")
}

func main() {
    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    stklog.CreateStack().SetName("main").End()
    log.AddHook(hook)
    log.Info("logs written to the 'main' stack")
    stklog.CreateStack().SetName("worker").End()
    worker()
}
```
Unfortunately, in this specific case, you can't go back to write to the "main" stack.
That's partly why we let you set your own unique ID per stack if you need to.
### Custom and optionals settings for Stacks
You can optionally interact with some properties of the Stack, that will help you to have a better control in `Stklog.io`.

#### SetRequestID
Let you chose a custom requestID, this _unique_ string is used to link logs to a Stack. It is sometimes useful to have control over it, in a microservice context for instance.

##### Behavior details
If this requestID already exists as one of your previously created stack (you called End()), we won't create a new one. However, any log sent with this requestID will be associated to the existing stack.
This behavior is on purpose, since, through a chain of microservices, you could pass your requestID from service to service and all your logs would be assign to the same Stack.

Edge case here, being that, if you create a stack with an ID that was used a week ago, we will use that specific stack on our side to link your logs ..
We recommand to use algorithms that ensure your ID will be unique. Moreover, your requestID need to be unique for this specific project key, it won't be mixed with other projects.

#### SetName
Giving a name to your stack, will be useful for Stklog.io UI. It will hopefully give you better insight about this Stack and what it is supposed to do.

#### SetFields
Let you give some "context" data to your stack, it will help you spot differences and hopefully debug or reproduce a potential bug. For instance, in case of an API or WebService, you could send clients HTTP Headers as Stack fields ..

#### Example
```go
package main

import (
    log "github.com/Sirupsen/logrus"
    stklog "github.com/stklog/logrus-stklog-hook"
)

func main() {
    hook := stklog.NewStklogHook("<project key>")
    defer hook.Flush()
    stklog.CreateStack()
        .SetName("test stack")
        .SetRequestID("test1234")
        .SetFields(map[string]interface{}{
           "Accept": "text/css,*/*;q=0.1",
           "Accept-Encoding": "gzip, deflate, sdch, br",
           "Referer": "https://stklog.io",
        }).End()
    log.AddHook(hook)
    log.Info("some logging message")
}
```
### Technical details
Specifically for Golang, we had to find a smart way to not ask you on which stack you want to write. We then decided to maintain an internal map[string]string, with key the current goroutine ID and with value the requestID of a stack. We invite you to check the code itself to understand our implementation.
### Miscellaneous
You can call `stklog.GetCurrentRequestID()` to know which requestID is associated to your current thread. It could help for debugging purpose if you do tricky things :)
