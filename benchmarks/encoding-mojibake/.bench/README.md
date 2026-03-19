# encoding-mojibake

## Difficulty
Easy

## Source
Community-submitted

## Environment
Java 17, Alpine Linux

## The bug
The CSV reader in `app/CsvReader.java` opens files using ISO-8859-1 charset instead of UTF-8. The input CSV is UTF-8 encoded, so multi-byte characters (accents, umlauts, CJK) are misinterpreted, producing mojibake in the output report.

## Why Easy
Single file, single line fix. The test output shows exactly which international names are corrupted. The charset declaration is explicit and the fix is a direct substitution to StandardCharsets.UTF_8.

## Expected fix
Change the charset from `Charset.forName("ISO-8859-1")` to `StandardCharsets.UTF_8` in the CsvReader constructor.

## Pinned at
Anonymized snapshot, original repo not disclosed
