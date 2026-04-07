# Engram-Native Simulation Platform

## Status

Draft product specification

## Working Name

Engram Simulation Cloud

## One-Line Thesis

Build a simulation platform where every persona, conversation, memory, and scenario runs natively on Engram's compression model so large synthetic populations can think, remember, disagree, and evolve at costs and latencies that make repeated simulation practical.

## Executive Summary

This product combines Engram's core advantage, key=value identity compression and compressed session history, with Cohort's strongest product ideas, persona generation, scenario orchestration, analytics, reports, and multi-agent discussion. The result is not a standard AI chat application. It is a persistent synthetic population platform where each persona carries a compact state representation, each round preserves memory without resending large prompts, and each simulation can branch, replay, compare, and scale.

The defining architectural decision is that everything revolves around the Engram service:

- Personas are stored and transmitted as compressed identity state.
- Scenario state is encoded as structured dimensions rather than prose-first prompts.
- Turn history is preserved as compressed session memory rather than flattened prompt text.
- Report and analytics jobs consume simulation state emitted by Engram sessions.
- Product economics are designed around prompt compression as a first-class system capability rather than an afterthought.

This document specifies the v1 product, the core platform primitives, the user workflows, the runtime architecture, the prompt-writing model, commercial framing, rollout phases, and the technical bets required to make the platform genuinely differentiated.

## Product Vision

### Vision Statement

Enable organizations to run high-fidelity synthetic population simulations that are persistent, scalable, explainable, and economically viable by making compressed state the native substrate of AI interaction.

### Strategic Goal

Move Engram from a developer-side optimization utility to a foundational runtime for agentic and simulation products.

### What Changes the Game

Most AI simulation tools rebuild large persona prompts on every turn, resend too much history, and treat memory as expensive narrative text. This product changes the operating model:

- identity becomes structured state
- memory becomes compressed session history
- prompt design becomes schema-driven rather than prose-driven
- cost scales sublinearly with conversation length
- repeated experimentation becomes viable

That combination is the moat.

## Problem

### Current Market Failure

Multi-persona and simulation tools are expensive, stateless, slow, and brittle because they repeatedly send:

- long persona biographies
- repeated behavioral instructions
- repeated scenario descriptions
- repeated conversation history
- repeated report-generation context

As the number of personas and turns grows, costs rise quickly and latency degrades. Teams compensate by shrinking prompts, shortening sessions, simplifying personas, or reducing the number of scenarios they test. That directly reduces realism and product value.

### User Pain

Organizations that want realistic simulation need all of the following at once:

- persona realism
- scenario persistence
- longitudinal memory
- repeated testing
- analytics and reporting
- cost control

The existing tooling forces tradeoffs between realism, scale, speed, and budget.

## Product Positioning

### Category

Synthetic population simulation platform

### Positioning Statement

For organizations that need realistic human simulation at scale, Engram Simulation Cloud is a compressed-state simulation platform that lets synthetic populations persist, evolve, and be analyzed over time. Unlike standard prompt-heavy chat systems, it runs on Engram-native compression so each simulation retains fidelity while using dramatically less context budget.

### Initial Beachhead Use Cases

1. Training simulation for public safety, healthcare, and regulated environments
2. Focus-group and message testing for communications and policy teams
3. Crisis rehearsal and tabletop exercises

## Core Product Principles

1. Engram-first runtime
   The platform must route every simulation turn through Engram-native session handling, not bolt compression on after prompt construction.

2. Structured state over narrative sprawl
   Prompts should be built from compact, typed fields wherever possible. Prose is reserved for content that truly benefits from rich language.

3. Persistent minds, not stateless replies
   Personas should preserve compressed identity, memory, and relationship state across turns and scenarios.

4. Cost transparency
   Users should see how compression changes cost, throughput, and scenario feasibility.

5. Analytics from the runtime
   The product should not only save tokens. It should learn which dimensions drive behavior, disagreement, drift, and scenario outcomes.

## Target Users

### Primary Users

- simulation designers
- training directors
- research and insights teams
- crisis planning teams
- product and messaging researchers

### Secondary Users

- analysts who review reports
- administrators who manage models, prompts, and budgets
- developers integrating the simulation engine into external systems

## Jobs To Be Done

1. Generate a realistic population for a scenario
2. Run a multi-turn simulation with persistent memory
3. Observe disagreement, alignment, and behavioral evolution over time
4. Compare alternate interventions across branched scenarios
5. Export insights, reports, and recommendations
6. Understand cost, token use, and runtime performance

## Product Scope

### In Scope for V1

- population generation
- scenario creation and branching
- Engram-native compressed persona state
- persistent multi-turn simulations
- roundtable and facilitator-led conversations
- compressed memory and relationship state
- analytics dashboards
- PDF and markdown reports
- admin controls for models, limits, and budgets
- API access for orchestration and exports

### Out of Scope for V1

- voice-first simulation
- realtime avatar rendering
- autonomous web-browsing personas
- open public marketplace of community simulations
- general-purpose agent platform unrelated to simulation

## Product Narrative

### The User Experience

The user creates a scenario, chooses a domain template, generates a synthetic population, and launches a simulation. Each persona is represented internally by an Engram identity profile and compact behavioral state. As the discussion proceeds, Engram preserves compressed history and evolving memory so the system does not resend large biographies and prior turns in full. The user can pause, fork the scenario, adjust a policy, rerun the same population, and compare outcomes. Reports and dashboards explain what changed, why it changed, how much it cost, and how stable the findings appear across runs.

## Core Objects

### Persona Identity

The durable identity specification for a synthetic participant.

Fields:

- persona_id
- tenant_id
- name
- demographic dimensions
- career dimensions
- behavioral dimensions
- communication style dimensions
- domain expertise dimensions
- background narrative
- constraint flags
- identity codebook version
- compressed identity payload

### Persona State

The mutable state of a persona during a simulation.

Fields:

- current stance
- confidence level
- stress level
- trust scores by peer
- memory references
- open questions
- scenario-relevant objectives
- last-turn summary
- drift indicators
- compressed state payload

### Scenario

The simulation context and objective space.

Fields:

- scenario_id
- title
- domain
- premise
- objectives
- artifacts and documents
- facilitator instructions
- evaluation rubric
- scenario codebook version

### Session

An Engram-native execution context for one simulation run or branch.

Fields:

- session_id
- tenant_id
- scenario_id
- branch_id
- participant set
- model policy
- context schema
- compressed history
- token and cost telemetry
- latency telemetry

### Branch

An alternate run derived from a prior session state.

Fields:

- branch_id
- parent_session_id
- fork_turn
- intervention delta
- branch notes

### Report

An analysis artifact produced from simulation telemetry and transcripts.

Fields:

- report_id
- type
- source sessions
- analysis findings
- export formats

## Engram-Centric Architecture

### Architectural Thesis

Every product subsystem should be downstream of Engram session primitives.

### Logical Architecture

```text
UI / API / SDK
  -> Simulation Orchestrator
    -> Engram Runtime Service
      -> Identity Compiler
      -> Context Codebook Manager
      -> Session Store
      -> Compression Engine
      -> Provider Router
      -> Telemetry Stream
    -> Simulation Modules
      -> Population Generator
      -> Scenario Engine
      -> Memory Engine
      -> Relationship Engine
      -> Analytics Engine
      -> Report Engine
```

### Required Evolution of Engram

Engram today is shaped like a local-first optimization service. To become the runtime center of this product, it must gain:

- multi-tenant session isolation
- durable session persistence beyond local ephemeral memory
- provider-agnostic routing beyond current proxy assumptions
- branchable session snapshots
- richer context schemas for persona and scenario state
- runtime telemetry APIs
- policy controls for budgets, rate limits, and model selection

### Required Evolution of Cohort Concepts

Cohort concepts should be absorbed as product modules on top of Engram, not as a separate runtime:

- persona generation becomes population generation on top of identity codebooks
- roundtable chat becomes session orchestration
- reports become telemetry-aware simulation reports
- RAG becomes selective context injection into compressed prompts
- analytics become session and branch comparison

## Prompt Model

### Product Bet

Prompt-writing must be redesigned around compression rather than merely compressed after the fact.

### Prompt Layers

1. Identity layer
   Compact key=value dimensions defining persona identity and durable traits

2. State layer
   Compact key=value dimensions defining transient state for the current session

3. Scenario layer
   Structured dimensions plus selective prose fragments describing the current situation

4. Retrieved context layer
   Minimal inserted evidence selected by scenario and turn relevance

5. Query layer
   The actual user or facilitator prompt for the turn

### Example

Traditional prompt style:

```text
You are Jamie, a 42-year-old paramedic with 17 years of field experience...
```

Engram-native style:

```text
id=name:jamie role:paramedic exp:17y stress:high comm:direct engage:skeptical
state=stance:concern trust.facilitator:0.4 trust.peer.a12:0.7 objective:protect_team
scenario=domain:hazmat phase:triage resource_level:constrained risk:unknown_exposure
query=Respond to the facilitator's revised protocol.
```

Rich prose still exists, but only for fields where narrative carries real value:

- biography snippets
- scenario artifacts
- policy text
- retrieved source material

### Prompt Authoring Rules

1. Every repeatable concept must have a typed field before it is allowed to live as prose.
2. Instructions should be split into stable identity, stable scenario, and dynamic turn state.
3. History should be passed as compressed session context, not recopied narrative whenever possible.
4. Long biographies should be summarized into identity dimensions plus one optional narrative section.
5. Analytics should capture which prompt fields materially affect outcomes.

## Product Modules

### 1. Population Studio

Purpose:
Generate synthetic populations for a domain and scenario.

Capabilities:

- cohort generation from templates or freeform descriptions
- diversity and coverage controls
- persona calibration and validation
- identity codebook compilation
- reusable population presets

### 2. Scenario Studio

Purpose:
Create structured scenario environments with objectives, artifacts, and branching conditions.

Capabilities:

- scenario builder
- artifact upload
- facilitator instructions
- branching rules
- evaluation rubric editor
- scenario codebook compilation

### 3. Simulation Engine

Purpose:
Run single-actor or multi-actor simulations on top of Engram sessions.

Capabilities:

- roundtable discussions
- facilitator-led simulations
- pairwise interactions
- branching and replay
- run scheduling
- model policy routing

### 4. Memory Engine

Purpose:
Maintain compressed persona and relationship state across turns and runs.

Capabilities:

- compressed memory summaries
- relationship state updates
- per-persona state transitions
- drift detection
- recall controls

### 5. Analytics Console

Purpose:
Expose the behavior and performance of the simulation.

Capabilities:

- stance analysis
- convergence and divergence tracking
- branch comparison
- prompt sensitivity analysis
- token, cost, and latency dashboards
- compression impact dashboards

### 6. Report Engine

Purpose:
Generate consumable artifacts from simulation outcomes.

Capabilities:

- executive summaries
- persona breakdowns
- scenario comparison reports
- intervention impact reports
- PDF and markdown export

### 7. Runtime Admin

Purpose:
Provide operational control over models, costs, policies, and tenant isolation.

Capabilities:

- model configuration
- tenant policies
- budget caps
- session retention controls
- audit and access logs

## Key User Workflows

### Workflow 1: Create and Run a Simulation

1. User selects domain template
2. User defines simulation objective
3. User uploads documents or context
4. Platform generates a population
5. Engram compiles identity codebooks
6. User launches simulation
7. Engram opens sessions for each participant
8. Simulation proceeds with compressed state updates
9. User reviews live analytics
10. User exports report

### Workflow 2: Branch a Scenario

1. User opens a completed or active simulation
2. User forks at turn N
3. User changes one intervention or condition
4. Platform reuses prior compressed state up to the branch point
5. New branch runs from the fork
6. Analytics compare outcomes against the original

### Workflow 3: Prompt Optimization

1. Admin reviews token and latency dashboard
2. System identifies high-cost prompt fields
3. Admin converts repeated prose into schema fields
4. Engram compiles a revised codebook version
5. Simulation quality and cost are compared before and after

## Functional Requirements

### Population Generation

- Generate personas from domain templates and freeform scenario descriptions
- Support configurable count, diversity, and role mix
- Produce both human-readable persona cards and compressed identity payloads
- Validate persona realism against domain constraints

### Scenario Management

- Create, edit, duplicate, archive, and branch scenarios
- Attach structured objectives, rules, and documents
- Store both authoring form and compiled compressed representation

### Session Runtime

- Start, pause, resume, branch, and terminate sessions
- Support one-to-one, one-to-many, and many-to-many interactions
- Persist compressed history and persona state per session
- Permit provider fallback while preserving session continuity

### Memory and State

- Maintain per-persona state updates after each turn
- Maintain peer relationship summaries
- Expose configurable retention windows
- Permit snapshotting and replay

### Analytics

- Track tokens, costs, compression rates, and p50/p95 latency
- Track stance, sentiment, disagreement, and movement over time
- Compare branch outcomes
- Surface drift and instability warnings

### Reporting

- Generate tenant-level and scenario-level reports
- Export markdown, PDF, and machine-readable JSON
- Include runtime cost and compression metrics alongside findings

### Administration

- Tenant-level quotas and rate limits
- RBAC for admins, analysts, and operators
- Audit trail of scenario and model changes

## Non-Functional Requirements

### Performance

- First-token latency target for standard turns: under 1.5 seconds at p50
- Full-turn latency target for 5-persona roundtable: under 8 seconds at p50
- Session rehydration from snapshot: under 500ms

### Reliability

- No loss of session state across worker restarts once persistence is enabled
- Degraded-mode fallback when compression features fail
- Provider routing fallback for transient upstream failures

### Security

- strict tenant isolation
- encrypted at-rest session persistence
- auditable access to transcripts and reports
- configurable retention and deletion policies

### Explainability

- show what codebook version drove a session
- show what scenario and identity fields were active
- show branch differences and runtime metrics

## Data Model Strategy

### Canonical Source of Truth

For each core object, store both:

- authoring representation
- compiled Engram-native representation

### Example Storage Pattern

Persona:

- `persona_source_json`
- `persona_identity_compiled`
- `persona_identity_codebook_version`

Scenario:

- `scenario_source_json`
- `scenario_compiled_context`
- `scenario_codebook_version`

Session:

- `session_metadata`
- `compressed_history_blob`
- `state_snapshot_blob`
- `telemetry_events`

This prevents lock-in to one serialization while making Engram-native execution first-class.

## API Surface

### Public API Domains

- `/populations`
- `/personas`
- `/scenarios`
- `/sessions`
- `/branches`
- `/analytics`
- `/reports`
- `/admin/runtime`

### Example APIs

`POST /populations/generate`

- input: training objective, domain, count, constraints
- output: population summary plus compiled identity profiles

`POST /sessions`

- input: scenario_id, population_id, runtime policy
- output: session_id, participant sessions, expected budget

`POST /sessions/{id}/fork`

- input: fork_turn, intervention_delta
- output: branch_id, new_session_id

`GET /sessions/{id}/metrics`

- output: token use, compression rate, latency, branch health

## Runtime Telemetry

### Why It Matters

The product should sell not only realism but also simulation economics. Telemetry is therefore a product feature, not only an ops concern.

### Required Metrics

- tokens in
- tokens out
- tokens saved by identity compression
- tokens saved by history compression
- tokens saved by state compression
- cost by session
- cost by branch
- latency by turn
- latency by provider
- compression failure rate
- prompt drift indicators

### User-Facing Views

- live run dashboard
- branch comparison dashboard
- population-level cost dashboard
- prompt optimization dashboard

## Business Model

### Pricing Thesis

This product can price on value created by simulation throughput, not only raw token pass-through.

### Pricing Components

- platform subscription by seat or tenant tier
- simulation runtime credits
- premium analytics and reporting tier
- enterprise private deployment tier

### Commercial Advantage

If Engram materially reduces per-simulation cost, margin expands or more simulations fit under the same customer budget. Both are strategically useful:

- higher gross margin at current price points
- disruptive price points against prompt-heavy competitors
- more scenario runs per customer per month

## Differentiators

1. Engram-native compressed-state runtime
2. Persistent persona memory at practical cost
3. Branchable scenario execution
4. Compression-aware analytics and prompt design
5. Structured-state authoring model instead of pure prose prompting

## Risks

### Product Risks

- Users may not immediately understand why compressed-state architecture matters.
- The value proposition can sound infrastructural rather than outcome-oriented.
- Prompt rewrites may be required before gains feel dramatic.

### Technical Risks

- Engram currently needs multi-tenant and durable-state evolution.
- Branching semantics are more complex than simple chat replay.
- Compression errors could be hard to debug without strong observability.
- Provider differences may limit uniform compression behavior.

### Mitigations

- Lead with simulation scale and persistence, not compression jargon.
- Ship clear telemetry that shows savings and throughput gains.
- Keep raw and compiled representations side by side for debugging.
- Start with one supported provider path if necessary, then expand.

## Success Metrics

### Product Metrics

- simulations run per tenant per month
- median turns per simulation
- branch runs per base scenario
- report exports per simulation
- monthly active simulation creators

### Runtime Metrics

- average input token reduction
- average session cost reduction
- p50 and p95 latency improvement
- session persistence hit rate
- branch reuse hit rate

### Outcome Metrics

- customer-reported realism score
- repeat usage by team
- time from scenario creation to first usable report

## Rollout Plan

### Phase 1: Engram Runtime for Simulation

Goal:
Use Engram as the LLM runtime behind a basic simulation workflow.

Deliverables:

- provider-agnostic Engram service path
- session persistence
- compressed identity support for personas
- basic roundtable simulation
- runtime cost dashboard

### Phase 2: Persistent Population Platform

Goal:
Make persona state and branchable sessions first-class.

Deliverables:

- per-persona mutable state
- relationship engine
- session snapshots
- scenario forking
- branch comparison analytics

### Phase 3: Optimization and Enterprise Readiness

Goal:
Make the system operable and commercially strong.

Deliverables:

- admin runtime controls
- tenant budgets and quotas
- enterprise reporting
- prompt optimization assistant
- deployment options

## MVP Definition

The MVP should prove one thing clearly: a simulation platform built around Engram-native compression can sustain richer persistent simulations at acceptable cost.

### MVP Use Case

Facilitator-led training simulation with 5 to 12 personas, 10 to 20 turns, persistent memory, and report generation.

### MVP Must-Haves

- population generation
- scenario authoring
- Engram-native session runtime
- compressed persona identity
- compressed history reuse
- branch-and-rerun
- analytics and PDF report

### MVP Nice-To-Haves

- fine-grained relationship modeling
- advanced sensitivity analysis
- multi-provider live routing

## Build Recommendation

Do not market the first version as a generic agent platform. Market it as a synthetic population simulation product with a visibly superior runtime model.

Do not implement Engram as only a proxy layer. Promote Engram to the system of record for session state, compression, and runtime telemetry.

Do not preserve Cohort's prompt-heavy design unchanged. Rewrite the prompt authoring model to maximize typed fields and minimize repeated prose.

## Open Questions

1. Should v1 optimize for training simulation or message-testing first?
2. Should session persistence live inside Engram storage or an external database with Engram serialization?
3. How much of persona state should remain human-readable versus fully compiled?
4. Should branch comparison be deterministic where possible, or intentionally stochastic with confidence bands?
5. Which provider path should be first-class at launch?

## Recommendation

Proceed with an Engram-native simulation product centered on one beachhead vertical, likely training simulation or focus-group testing. The product is strategically sound if Engram is elevated from optimization utility to runtime substrate. That is the move most likely to produce a product that feels materially different rather than incrementally cheaper.
