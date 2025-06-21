import re
import glob
import requests

# Regex pattern to extract URLs from markdown links [text](url)
pattern = re.compile(r'\[.*?\]\((.*?)\)')

links = []

# Iterate over all .md files in all directories inclusing childern
for filename in glob.glob("**/*.md",recursive=True):
    with open(filename, 'r', encoding='utf-8') as f:
        print(f"Processing file: {filename}")
        content = f.read()
        found_links = pattern.findall(content)
        links.extend(found_links)

print(f"Extracted links:{links}")

for link in links:
    try:
        if requests.head(link).status_code==200:
            print(f'{link} link is valid')
        else:
            print (f'{link} is not valid')
    except requests.exceptions.RequestException as e:
        print("The link has exceeded the dns resolution limit and failed")