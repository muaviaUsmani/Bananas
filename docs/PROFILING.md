# Profiling Bananas

This guide explains how to use Go's built-in profiling tools to analyze performance and identify bottlenecks in Bananas.

## Quick Start

All Bananas services expose pprof endpoints on separate ports:

- **API Server**: `http://localhost:6060/debug/pprof/`
- **Worker**: `http://localhost:6061/debug/pprof/`
- **Scheduler**: `http://localhost:6062/debug/pprof/`

You can customize ports with the `PPROF_PORT` environment variable.

## Available Profiles

### CPU Profile

Analyze CPU usage and identify performance bottlenecks:

```bash
# Collect 30 seconds of CPU profile from worker
go tool pprof http://localhost:6061/debug/pprof/profile?seconds=30

# In pprof interactive mode:
(pprof) top10          # Show top 10 functions by CPU time
(pprof) list FuncName  # Show source code for function
(pprof) web            # Generate call graph (requires graphviz)
(pprof) pdf > profile.pdf  # Export as PDF
```

**What to look for:**
- Functions consuming > 10% of CPU
- Unexpected hot paths
- String allocations in loops
- Inefficient serialization

### Memory Profile (Heap)

Analyze current memory allocations:

```bash
# Heap profile (currently allocated memory)
go tool pprof http://localhost:6061/debug/pprof/heap

# In pprof interactive mode:
(pprof) top10 -cum     # Show top allocators (cumulative)
(pprof) list FuncName  # Show allocations in function
(pprof) web            # Visualize memory allocations
```

**What to look for:**
- Large allocations (> 1MB)
- Growing allocations over time (memory leaks)
- Unexpected caching behavior

### Allocation Profile

Analyze all allocations (even if freed):

```bash
# Track all allocations (not just what's currently in heap)
go tool pprof http://localhost:6061/debug/pprof/allocs

(pprof) top10          # Functions doing most allocations
```

**What to look for:**
- High-frequency allocations in hot paths
- Opportunities for object pooling
- String/slice allocations that could be reduced

### Goroutine Profile

Analyze goroutine usage and potential leaks:

```bash
go tool pprof http://localhost:6061/debug/pprof/goroutine

(pprof) top10          # Show goroutine states
(pprof) traces         # Show goroutine stack traces
```

**What to look for:**
- Growing goroutine count (leaks)
- Blocked goroutines
- Expected count: ~worker_concurrency + a few system goroutines

### Block Profile

Analyze blocking operations (mutex, channel ops):

```bash
go tool pprof http://localhost:6061/debug/pprof/block
```

**What to look for:**
- Long-duration blocks
- Lock contention
- Channel send/receive delays

### Mutex Profile

Analyze lock contention:

```bash
go tool pprof http://localhost:6061/debug/pprof/mutex
```

**What to look for:**
- Hot mutexes (frequently locked)
- Long hold times
- Opportunities for lock-free data structures

## Common Profiling Workflows

### Investigating High CPU Usage

```bash
# 1. Take CPU profile during high load
go tool pprof http://localhost:6061/debug/pprof/profile?seconds=30

# 2. Find hot functions
(pprof) top20 -cum

# 3. Examine specific function
(pprof) list github.com/muaviaUsmani/bananas/internal/queue.(*RedisQueue).Enqueue

# 4. Generate visualization
(pprof) web
```

### Investigating Memory Leaks

```bash
# 1. Take heap snapshot before load
go tool pprof -proto http://localhost:6061/debug/pprof/heap > heap_before.pb.gz

# 2. Run workload for 5 minutes

# 3. Take heap snapshot after load
go tool pprof -proto http://localhost:6061/debug/pprof/heap > heap_after.pb.gz

# 4. Compare snapshots
go tool pprof -base heap_before.pb.gz heap_after.pb.gz

# 5. Look for growing allocations
(pprof) top10 -cum
```

### Investigating Goroutine Leaks

```bash
# 1. Check goroutine count before load
curl http://localhost:6061/debug/pprof/goroutine?debug=1 | grep "goroutine profile"

# 2. Run workload

# 3. Check goroutine count after load
curl http://localhost:6061/debug/pprof/goroutine?debug=1 | grep "goroutine profile"

# If count is growing, investigate:
go tool pprof http://localhost:6061/debug/pprof/goroutine
(pprof) top10
(pprof) traces
```

## Continuous Profiling in Production

For production environments, consider using:

- **Datadog Continuous Profiler**: Automatic profiling with minimal overhead
- **Grafana Pyroscope**: Open-source continuous profiling
- **Google Cloud Profiler**: Free profiling for GCP workloads

## Performance Investigation Checklist

When investigating performance issues:

1. ✅ **Establish baseline**: Run benchmarks before optimization
2. ✅ **Take CPU profile**: Identify hot paths
3. ✅ **Take heap profile**: Identify memory bottlenecks
4. ✅ **Check goroutine count**: Rule out goroutine leaks
5. ✅ **Examine Redis operations**: Use Redis SLOWLOG
6. ✅ **Make targeted changes**: Optimize one thing at a time
7. ✅ **Benchmark after**: Verify improvement
8. ✅ **Document findings**: Record what worked

## Tips and Best Practices

### Profiling Best Practices

- **Profile under realistic load**: Use production-like workloads
- **Profile for 30+ seconds**: Short profiles may miss patterns
- **Use `-cum` flag**: Shows cumulative time including callees
- **Compare before/after**: Always measure impact of changes
- **Profile in production**: Development loads may not match prod

### Common Pitfalls

❌ **Don't profile in test mode**: Use production builds
❌ **Don't optimize without profiling**: Measure first!
❌ **Don't profile cold starts**: Let system warm up
❌ **Don't ignore the 80/20 rule**: Focus on hot paths
❌ **Don't make multiple changes**: Change one thing at a time

### Reading pprof Output

```
flat  flat%   sum%        cum   cum%
100ms 10.00% 10.00%     500ms 50.00%  FuncA
 50ms  5.00% 15.00%     200ms 20.00%  FuncB
```

- **flat**: Time spent in function itself (not callees)
- **cum**: Cumulative time including all callees
- Focus on high **cum%** for biggest impact

## Profiling Examples

### Example 1: Finding Slow Serialization

```bash
$ go tool pprof http://localhost:6061/debug/pprof/profile?seconds=30

(pprof) top10 -cum
    flat  flat%   sum%        cum   cum%
     0     0%     0%      5.2s  52.00%  json.Marshal
   1.2s  12.00% 12.00%    3.8s  38.00%  json.(*encodeState).marshal
   0.5s   5.00% 17.00%    2.1s  21.00%  json.(*encodeState).reflectValue

# Action: Replace JSON with protobuf for 3-5x speedup ✅
```

### Example 2: Finding Memory Leak

```bash
$ go tool pprof http://localhost:6061/debug/pprof/heap

(pprof) top10 -cum
    flat  flat%   sum%        cum   cum%
    0     0%     0%     800MB 80.00%  queue.(*RedisQueue).resultCache
  500MB 50.00% 50.00%   500MB 50.00%  lru.(*Cache).Add

# Action: Reduce cache size or add TTL ✅
```

## Useful pprof Commands

```bash
# Top consumers
top10
top20 -cum

# List function source
list FuncName

# Show callgraph for function
peek FuncName

# All callers of function
callers FuncName

# All callees of function
callees FuncName

# Generate visualization
web
pdf > output.pdf

# Interactive web UI
go tool pprof -http=:8080 profile.pb.gz
```

## Further Reading

- [Go pprof Documentation](https://pkg.go.dev/net/http/pprof)
- [Profiling Go Programs](https://go.dev/blog/pprof)
- [Go Performance Workshop](https://github.com/davecheney/high-performance-go-workshop)

---

*Last Updated: 2025-10-24*
