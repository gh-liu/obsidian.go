---
id: "1774849813-NCZI"
title: "DOD"
tags: [BN]
aliases: ["DOD", "Data-Oriented Design"]
createdAt: 2026-03-30
updatedAt: 2026-03-30 14:06:09
---

# The Art of Data-Oriented Design

> Software design should center on **data layout and transformation**, not object modeling.

## Core Ideas

Two fundamental principles:

1. **Data is not the problem domain** — meaning is what we assign.
2. **Data is type, frequency, quantity, shape, and probability** — real runtime statistics drive algorithm choice.

## Key Insights

### Relational Model for Game Data

Normalize complex object graphs into flat arrays. 1NF → BCNF eliminates NULLs, duplicates, multi-valued fields.

### Component → Manager → Entity Disappears

The entity becomes nothing more than a collection of components.

## Quotes

> When you want a banana, you get the gorilla holding the banana and the entire jungle.
