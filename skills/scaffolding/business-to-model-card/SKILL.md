---
name: business-to-model-card
description: Use this skill when the input is product/business narrative and the goal is to produce executable domain modeling artifacts before YAML coding. It converts descriptions into model cards (context, aggregate, relations, state machine, validation, actions, events, custom overlays), enforces single-writer/cross-domain event rules, creates refine task lists, and prepares handoff inputs for `ubo-yaml-writing`.
---

# Business to Model Card

## Purpose

Use this skill to bridge the gap between:
1. Narrative business requirements.
2. Concrete modeling decisions.
3. YAML implementation handoff.

This skill is for principle + structure.  
Use `ubo-yaml-writing` after this skill for schema-level authoring.

## When To Trigger

Trigger when user asks any of:
1. "How to model this business flow?"
2. "Given product PRD/business description, produce domain model."
3. "Need model card/template/refine checklist before coding."
4. "Need to clarify core vs vertical vs tenant custom boundaries."

## Core Rules

1. Do not start from YAML. Start from model boundary and action semantics.
2. Reuse existing core aggregate when available; do not create duplicate aggregate for lifecycle phase.
3. Draft is usually a state (for example `status=draft`), not a new aggregate.
4. Cross-domain linkage goes through events, not direct writes to other domains' source-of-truth.
5. Single-writer rule must be explicit for critical facts.

## Workflow

### Step 1: Normalize input

Extract:
1. Business objective.
2. Actors and roles.
3. Facts that must be persisted.
4. Lifecycle transitions.
5. Tenant-specific differences.

### Step 2: Define model boundary

Produce:
1. Context responsibility / non-responsibility.
2. Aggregate root and single write entry.
3. Explicit "reuse existing model" note if applicable.

### Step 3: Build action semantics

For each action define:
1. Input.
2. Preconditions.
3. Effects (state/data/events).
4. Idempotency key.
5. Failure modes.

### Step 4: Define validation and invariants

Three layers:
1. Input validation.
2. Business precondition validation.
3. Post-action invariant.

### Step 5: Define cross-domain events

For each event define:
1. Publisher.
2. Trigger.
3. Consumers.
4. Consumer-side idempotency.

### Step 6: Define custom overlays

Must include:
1. Overlay points (field/rule/node/template).
2. Priority: `tenant custom > vertical > core`.
3. Same-layer conflict strategy.
4. Fallback path.

### Step 7: Emit artifacts

Always output:
1. Model Card (full).
2. Quality Gate mini-score.
3. Refine task list (modular).
4. YAML handoff checklist for `ubo-yaml-writing`.

## Output Contract

### A. Model Card (required sections)

1. Meta: model id, module, context, type.
2. Boundary: responsible/not responsible, source-of-truth, single writer.
3. Data: aggregate/entities/value objects/relations.
4. State machine.
5. Action list.
6. Validation and invariants.
7. Events.
8. Custom overlay points.
9. Core mapping and gaps.
10. Test mapping.

### B. Quality Gate mini-score (0-2 each)

1. Boundary clarity.
2. State-machine completeness.
3. Constraint strength.
4. Custom parameterization.
5. Core mapping completeness.

Pass condition:
1. Total >= 7/10.
2. Key dimensions (state/constraint/mapping) cannot be 0.

### C. Refine task list

Each task must include:
1. Scope.
2. Expected artifact.
3. Exit criteria.

### D. YAML handoff checklist

Before handing off to `ubo-yaml-writing` ensure:
1. Aggregate and relation targets are stable.
2. Action input/precondition/effects are stable.
3. Invariant is testable.
4. Overlay conflict strategy is fixed.

## Quick Decision Heuristics

1. "Need draft form save" -> add `save_draft` action on existing aggregate; keep weak validation.
2. "Need final submit" -> `submit_*` action with strong validation and invariant checks.
3. "Need cooperation with another domain" -> publish/consume events; no direct cross-domain fact write.
4. "Need customer difference" -> parameterize in custom overlays; no model copy-per-tenant.

