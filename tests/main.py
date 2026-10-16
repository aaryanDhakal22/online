"""Entry point for the legacy interactive runner. Delegates to test.py for
the full menu, or to prod_test.one_test() for a single-shot smoke check.
"""

import os
import sys

from prod_test import one_test


def main():
    app_env = (os.getenv("APP_ENV") or "dev").lower()
    if app_env == "dev_one":
        print("Sending one test order")
        one_test()
        return
    if app_env in ("dev", "prod"):
        # Hand off to the interactive harness.
        import test  # noqa: F401  — imported for its side effects (the menu loop)
        return
    print(f"Invalid APP_ENV: {app_env!r}", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
