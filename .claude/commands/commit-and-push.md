# Commit and Push

Commit all staged and unstaged changes, run quality checks, and push to the remote.

## Instructions

Follow these steps precisely and in order. Do not skip steps. If any step fails, stop and fix the issue before continuing.

### 1. Assess Changes

Run these commands in parallel to understand the current state:

```bash
git status
git diff
git diff --cached
git log --oneline -10
```

Review all changes (staged and unstaged) to understand what is being committed.

### 2. Quality Checks

Run the full quality pipeline before committing. If any check fails, fix the issue and re-run.

```bash
make fmt
```

Stage any formatting changes, then run:

```bash
make check
```

This runs `vet`, `lint`, and `test` in sequence. All must pass before proceeding.

### 3. README Update Check (MANDATORY)

**STOP**: You MUST complete this step before proceeding to commit.

- **Read the README** to understand current documentation
- **Compare changes against README content** — for each changed file, check if:
  - New commands, features, or functionality were added
  - Installation steps or prerequisites changed
  - Directory structure or file locations changed
  - CLI options, flags, or configuration changes need documenting
- **If ANY documentation updates are needed**:
  - Update the README BEFORE creating the commit
  - Stage the README changes along with the other changes
- **If unsure**: Ask the user whether README updates are needed
- **Do NOT skip this step** — documentation drift causes confusion

### 4. CLAUDE.md Update Check

- **Read CLAUDE.md** and compare against the changes being committed
- Check if any of the following changed:
  - Architecture or package structure
  - Key patterns or conventions
  - Available Makefile targets or common commands
  - Internal package interfaces or responsibilities
- **If updates are needed**: Update CLAUDE.md and stage the changes
- **If unsure**: Ask the user

### 5. CHANGELOG Update (User-Facing Changes Only)

**Applies to commits with these prefixes**: `fix:`, `feat:`, `add:`, `update:`, `breaking:`

If the commit is purely internal (`docs:`, `chore:`, `refactor:`, `test:`), skip this step.

#### 5a. Tidy Existing Entries

Before adding anything new, check `CHANGELOG.md` for entries under `[Unreleased]` that belong to an already-released version:

```bash
git tag --sort=-v:refname
git log --oneline v0.1.0..HEAD  # adjust tag as needed
```

Cross-reference entries under `[Unreleased]` with the tagged commits. Move any misplaced entries into their correct `[x.y.z]` section (creating the section if needed).

#### 5b. Add New Entry

Add the new entry under the `[Unreleased]` section using this format:

```markdown
## [Unreleased]

### Added
- New feature or capability

### Changed
- Enhancement to existing feature

### Fixed
- Bug fix description

### Removed
- Removed feature or capability
```

If `CHANGELOG.md` does not exist, create it with this structure:

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/).

## [Unreleased]

### Added
- Entry here
```

Stage the CHANGELOG changes.

### 6. Stage and Commit

Stage all relevant files by name (do NOT use `git add -A` or `git add .`):

```bash
git add <specific files>
```

Create the commit using the project's commit message conventions:

- **Prefix**: lowercase with colon — `add:`, `fix:`, `feat:`, `update:`, `breaking:`, `refactor:`, `test:`, `docs:`, `chore:`
- **Style**: lowercase, concise, imperative mood
- **No trailing period**
- **No Co-Authored-By lines**
- **No mention of Claude or AI**

Use a heredoc for the message:

```bash
git commit -m "$(cat <<'EOF'
prefix: concise description of the change
EOF
)"
```

Version bump reference (for choosing the right prefix):
- `breaking:` → major (x.0.0)
- `add:` / `update:` → minor (0.x.0)
- `feat:` / `fix:` → patch (0.0.x)
- `docs:` / `chore:` / `refactor:` / `test:` → no release

### 7. Push

**Non-trivial changes MUST go through a pull request.** Only `docs:` and `chore:` commits may be pushed directly to main. For all other prefixes (`feat:`, `fix:`, `add:`, `update:`, `breaking:`, `refactor:`, `test:`), create a new branch and open a PR instead of pushing directly:

```bash
# For docs:/chore: — push directly
git push -u origin HEAD

# For all other prefixes — branch and PR
git checkout -b <branch-name>
git push -u origin HEAD
gh pr create --title "prefix: description" --body "$(cat <<'EOF'
## Summary
- Description of changes
EOF
)"
```

### 8. Verify

Run `git status` to confirm a clean working tree and successful push.

Report the commit hash and any version bump that will be triggered by the commit prefix.
