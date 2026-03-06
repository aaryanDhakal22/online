from client import ApiClient
from prod_test import one_test

def main():
    app_env = os.getenv("APP_ENV")
    if app_env == "dev":
        print("Running in dev mode")
    elif app_env == "prod":
        print("Running in prod mode")
    elif app_env == "dev_one":
        print("Sending one test order")
        one_test()
    else:
        print("Invalid app env")

if __name__ == "__main__":
    main()
