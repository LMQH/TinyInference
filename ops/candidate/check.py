#!/usr/bin/env python3
"""Confirm the previous deployment remains isolated during candidate QA."""

from start import check_old_isolated


if __name__ == "__main__":
    check_old_isolated()
    print("previous web/API/controller remain paused with their recorded identities")
