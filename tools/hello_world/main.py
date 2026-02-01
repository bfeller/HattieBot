import json
import sys

if __name__ == "__main__":
    data = json.load(sys.stdin)
    name = data.get("name", "World")
    print(f"Hello, {name}!")
