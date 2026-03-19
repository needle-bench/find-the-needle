# kernel-panic-ioctl

## Difficulty
Hard

## Source
Community-submitted

## Environment
C (gcc, musl-dev), Alpine Linux

## The bug
The ioctl handlers in `src/ioctl_handler.c` have inconsistent pointer validation. The SET_CONFIG handler validates the key pointer but not the value pointer or value_len bounds. The GET_CONFIG handler validates the key but not the output buffer pointer. Crafted inputs with NULL value pointers or oversized lengths cause segfaults (simulated kernel panics).

## Why Hard
Requires systematic analysis of all pointer dereference paths in the ioctl handler. The self-test passes because it uses valid inputs, so the agent must reason about adversarial inputs. Multiple validation gaps exist across two separate handler functions, requiring the agent to identify each missing check. The kernel driver pattern (validate before dereference) demands security-oriented thinking.

## Expected fix
Add NULL checks for `req->value` and bounds checks for `req->value_len` in the SET_CONFIG handler, and add a NULL/zero-length check for the output buffer in the GET_CONFIG handler.

## Pinned at
Anonymized snapshot, original repo not disclosed
