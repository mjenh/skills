# Team artifact layout — slug folders

All BMad planning and implementation artifacts for a product live under a **slug folder** derived from the PRD title (preferred) or project name.

## Slug derivation

1. Take the PRD title from frontmatter or the agreed product name.
2. Lowercase, trim, replace spaces/underscores with hyphens, strip non-alphanumeric except hyphens.
3. Collapse repeated hyphens; trim leading/trailing hyphens.

Examples:

| Title / name   | Slug          |
|----------------|---------------|
| personal AI    | `personal-ai` |
| My Cool App    | `my-cool-app` |
| agent          | `agent`       |

Once established during PRD (or GDD / game brief) creation, **reuse the same slug** for every downstream workflow in that product line.

## Resolving slug at activation

Before reading or writing artifacts:

1. If the user names a slug or product, use it.
2. Else scan `{planning_artifacts}/*/` for an existing slug folder containing a `*-prd.md`, `*-gdd.md`, or `*-brief.md`.
3. If exactly one match, use it.
4. If multiple matches, ask the user which product.
5. If none, derive from PRD/GDD title during the creating workflow and create the folder.

Store the resolved slug in working memory for the session. Write `{planning_artifacts}/{slug}/.artifact-slug` (single line, the slug) when first creating the folder so later workflows can read it.

## Directory layout

```
{output_folder}/
  planning-artifacts/
    {slug}/
      {slug}-prd.md              # BMM PRD (primary deliverable)
      {slug}-brief.md            # product / game brief
      {slug}-prfaq.md            # PRFAQ when used
      {slug}-architecture.md     # architecture spine / doc
      {slug}-ux.md               # UX spine (legacy single-file)
      DESIGN.md / EXPERIENCE.md  # UX spine pair (bmad-ux / gds-ux), inside slug folder
      {slug}-gdd.md              # GDS game design doc
      {slug}-epics.md            # epics + stories
      {slug}-narrative.md        # narrative design (GDS)
      sprint-status.yaml         # canonical name (not slug-prefixed)
      .artifact-slug             # optional marker file
      .memlog.md / decision-log.md / addendum.md  # workflow working files stay beside deliverables
  implementation-artifacts/
    {slug}/
      {story-key}.md             # user stories (e.g. 1-2-auth.md)
      epic-{n}-retro-{date}.md   # retrospectives
      spec-{intent-slug}.md      # quick-dev specs
      investigations/            # bmad-investigate case files
        {ticket-slug}-investigation.md
      tests/                     # QA / test summaries when generated
```

## Path variables (override SKILL defaults)

When a workflow defines paths, substitute:

| Default pattern | Slug-scoped replacement |
|-----------------|-------------------------|
| `{planning_artifacts}/*prd*.md` | `{planning_artifacts}/{slug}/{slug}-prd.md` |
| `{planning_artifacts}/epics.md` | `{planning_artifacts}/{slug}/{slug}-epics.md` |
| `{planning_artifacts}/*architecture*.md` | `{planning_artifacts}/{slug}/{slug}-architecture.md` |
| `{planning_artifacts}/*ux*.md` | `{planning_artifacts}/{slug}/` (DESIGN.md, EXPERIENCE.md, or `{slug}-ux.md`) |
| `{planning_artifacts}/*gdd*.md` | `{planning_artifacts}/{slug}/{slug}-gdd.md` |
| `{implementation_artifacts}/sprint-status.yaml` | `{planning_artifacts}/{slug}/sprint-status.yaml` |
| `{implementation_artifacts}/{story-key}.md` | `{implementation_artifacts}/{slug}/{story-key}.md` |
| `{implementation_artifacts}/epic-*-retro-*.md` | `{implementation_artifacts}/{slug}/epic-*-retro-*.md` |

## Primary deliverable filenames

Workflows that default to generic names (`prd.md`, `gdd.md`, `brief.md`) must use slug-prefixed deliverables at the slug folder root:

- PRD → `{slug}-prd.md`
- GDD → `{slug}-gdd.md`
- Brief → `{slug}-brief.md`
- Architecture → `{slug}-architecture.md`
- Epics → `{slug}-epics.md`
- Narrative → `{slug}-narrative.md`

Working/scratch files (`.memlog.md`, `addendum.md`, `decision-log.md`, `.working/`, `review-*.md`) may remain canonical names inside the slug folder.

## Multi-product repos

Each product gets its own slug folder. Never write planning or implementation artifacts at the roots of `planning-artifacts/` or `implementation-artifacts/` without a slug subfolder.
