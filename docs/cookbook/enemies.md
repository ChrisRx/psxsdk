---
title: Enemies
weight: 6
description: The few lines most games' enemies were made of: chasing and fleeing, bouncing, wandering, set patterns, random jitter, a sine wave, and states to switch between them.
---

Most enemies in most games are a few lines each. The SCEE tutorial's chapter 8 has seven, which the [ai](../examples/ai.md) example puts in one program, chosen with L1 and R1.

## Chasing and fleeing

An enemy that chases steps towards the player, a pixel in each direction it is off by, and one that flees steps away:

```go src=../../examples/ai/main.go#chase,flee
```

It is not clever, it goes straight for the player and never round anything, but it is what most chasing in 2D games was. A fleeing enemy is kept on the screen, or it leaves it.

## Moving on its own

Bouncing reverses a speed at each edge. Wandering goes one way until it meets an edge, then picks another at random. A set pattern reads its moves from a table, one a step, the way a game scripted a dive or a boss's dance:

```go src=../../examples/ai/main.go#followPattern
```

Flies move a random small amount every frame, which reads as buzzing, and a wave swings an enemy from side to side by the sine of the frame count as it falls.

## States

A game puts behaviors like these behind a state, which the enemy changes when something happens: it sees the player, it is hit, a timer runs out. Here the state changes at random every few seconds, the start of every game's state machine:

```go src=../../examples/ai/main.go#changeStates
```

The enemy's state is one variable, and each frame does what that state says. Adding a behavior is adding a state, and the conditions for leaving it.

<!-- player ai -->

## Random numbers

`Rand` is the C library's, the same sequence every time from the same `Srand`. That makes a game's randomness repeat exactly from one run to the next unless it is seeded from something that varies, such as the frame the player first pressed start on. The repeat is useful in testing, which is why the example seeds with a constant.
