# Building a custom gote skill

Instructions for an agent helping a user turn `SKILL.md` in this directory into their own gote skill. The user wants a skill that carries all the important gote facts (the core section) plus their own conventions (the CUSTOMIZE blocks).

## Steps

1. Ask the user for a skill name (lowercase, hyphens). Copy the template:

   ```
   mkdir -p ~/.claude/skills/<name>
   cp skill/template/SKILL.md ~/.claude/skills/<name>/SKILL.md
   ```

   Set `name:` to `<name>` and write the `description:` (see question 0).
2. Inspect the vault before asking anything, so you can propose defaults instead of asking open questions. Use `--json` and read only:
   - `gote status --json` for note and link counts.
   - `gote tag --json` for the tag vocabulary in use.
   - `gote search -t --json created:26` (filter-only queries list newest first) and `gote orphans --json` for shape.
   - `ls` the top-level directories under `notes_dir` (`gote config` shows it); `ls ~/.config/gote/templates/`.
   - Read 3 to 5 representative notes from disk (a daily note, a project note, something tagged) for frontmatter, section headings, link style and naming.
3. Interview the user, one question at a time, in the order below. For each, state what you saw in the vault and propose a default; accept "yes" or a correction. Replace the matching `<!-- CUSTOMIZE: <block> ... -->` comment with plain prose (and a table where it helps). Keep it short and imperative: the final skill is instructions to an agent.
4. Never edit text between `GOTE-CORE:START` and `GOTE-CORE:END`, except to replace it wholesale with the current upstream version when updating. Put everything user-specific in the CUSTOMIZE areas or in new sections outside the core.
5. Delete any block the user says does not apply (write nothing rather than inventing rules). Delete the leftover guidance comment at the top.
6. Finish by showing the user the resulting file path and its customized sections (not the core), and ask what to change. Do not claim it is done until they accept.

## Questions

0. Skill description: when should an agent reach for this skill? (Words the user says: "my notes", "journal", "project log".) Becomes the frontmatter `description:`.
1. `layout`: which directories exist, what goes in each, and where does a new note with no obvious home go? Default: what `ls` showed; inbox if one exists, else the root.
2. `note-types`: what kinds of notes do you write, and which gote template does each use? Default: group the sampled notes by tag and directory; list `gote template list` output.
3. `tags`: which tags are allowed, how many per note, any naming rules? Default: the top tags from `gote tag --json`.
4. `naming`: how are files and titles named; do you use dated prefixes or aliases? Default: the pattern in the sampled filenames.
5. `daily`: do you keep a daily note, where, with which sections, and how do unfinished tasks carry over? Default: `daily/<yymmdd>_daily.md` if present.
6. `create-vs-append`: when should the agent make a new note versus append versus edit existing text? Default: append to logs, create for new topics, edit prose only on request.
7. `linking`: do you use hub notes, `parent:`, `related:`, or inline `[[links]]` only; should new notes always link somewhere? Default: whatever the sampled notes do; check `gote orphans --json` for tolerance of orphans.
8. `permissions`: what may the agent change without asking, and what must it ask about (rename, delete, bulk edits, existing prose, frontmatter)? Default: ask for rename, delete and edits to existing prose.
9. `integrations`: which other tools relate to the notes (calendar, issue tracker, task manager, git) and how does a note map to each? Skip if none; never invent an integration.
10. `reporting`: how should the agent report back (length, tone, citation format, list of files touched)? Default: terse, `id:line` citations, files touched listed.

## Rules for the result

- Every claim about gote belongs to the core section. Do not restate or contradict it elsewhere; if the user wants something gote cannot do, say so and record a workaround in the relevant block.
- Examples in CUSTOMIZE blocks are illustrations. Replace them with the user's real values or remove them.
- Do not run commands that open a menu or browser (`view`, menu aliases) while exploring. `new`, `open` and `today` create notes, so do not run them during the build either.
- Do not edit, create or rename notes in the user's vault during the build.

## Updating an existing skill

When gote changes, take the upstream `skill/template/SKILL.md`, copy everything between its `GOTE-CORE` markers over the same region in the user's skill, and leave the rest. Verify with `gote help --json` that every command appears in the command table.
