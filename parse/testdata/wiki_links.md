---
id: "1770123039-URZR"
title: "Go: schedule"
tags: [CS, GO]
aliases: ["go-schedule"]
series: Inside Go
createdAt: 2026-03-21
updatedAt: 2026-03-22 00:07:46
---

# Go: schedule

> How do thousands of goroutines run efficiently on a handful of OS threads?

References: [[1774849813-NCZI|DOD]]
See also: [[1773826925-KAJG|observability]]

## GMP Model

### Core Idea

Goroutines are multiplexed onto OS threads via the GMP model.

### g - Goroutine

A goroutine is a lightweight user-space thread. [[#sudog---waiting-list|sudog]] is used for waiting.

### sudog - Waiting List

Represents a goroutine in a wait queue.

## Schedule Loop

### schedule() function

The core scheduling loop. See [[schedule-details]] for implementation.

### Preempt Based Signal

Uses signals for cooperative preemption.

## References

- [Go scheduler paper](https://golang.org/sched)
- [[1773846392-UKNM|kafka]] for async patterns
