# timezone-scheduling

## Difficulty
Easy

## Source
Community-submitted

## Environment
Python 3.12, pytz, Alpine Linux

## The bug
The scheduler in `app/scheduler.py` uses the UTC date (not the event's local date) when constructing the event's scheduled datetime. For events whose local time and UTC time fall on different calendar days (e.g., 23:30 US/Eastern = 04:30 UTC next day), the scheduler builds the wrong date and the event never matches the execution window.

## Why Easy
Single file fix with a clear pattern. The test output shows exactly which late-night events fail to fire. The date variable name (`today_utc` vs local date) makes the root cause apparent once found.

## Expected fix
Convert the window start time to the event's local timezone before extracting the date, so the scheduled datetime uses the correct local calendar day.

## Pinned at
Anonymized snapshot, original repo not disclosed
