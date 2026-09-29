#!/usr/bin/env bash
set -e
grep -Eq '^[[:space:]]*pnpm[[:space:]]+install([[:space:]]|$)' answer.txt &&
  ! grep -Eq '(^|[^[:alnum:]_])npm[[:space:]]+install([[:space:]]|$)' answer.txt
