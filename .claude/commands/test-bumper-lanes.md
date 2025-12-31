---
description: Test bumper-lanes threshold enforcement
argument-hint: "[parallel]"
model: haiku
---

# Test Bumper Lanes

<threshold>
!`jq -r '.threshold // 600' .bumper-lanes.json 2>/dev/null || echo 600` points
</threshold>

## Step 1: Create directories

Directories Initialized: !`mkdir -p tmp-threshold-test/src/components tmp-threshold-test/src/utils tmp-threshold-test/tests/unit`

## Step 2: Write files

Sequential by default. If `$ARGUMENTS` contains `parallel`, batch all Writes in one message.
!`ls`

```
./tmp-threshold-test/
├── README.md
├── src/
│   ├── index.ts
│   ├── components/
│   │   ├── Button.tsx
│   │   └── Modal.tsx
│   └── utils/
│       └── helpers.ts
└── tests/
    └── unit/
        └── button.test.ts
```

**Content pattern** (example for 75 lines):

The content of each files does not matter, but must have at least 75 lines. Use the following pattern:
```
// filename.ext
// Line 002
// Line 003
...
// Line 075
```

Write each file using the pattern above until we exceed the Threshold
