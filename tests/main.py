from client import ApiClient

def main():
    
    client = ApiClient()
    client.load_env()
    print(client.get("test"))



if __name__ == "__main__":
    main()
