import os, json

# os.system("wget https://datasets.imdbws.com/title.basics.tsv.gz")
# os.system("gunzip title.basics.tsv.gz")

# get the last read line of the IMDB file
f = open('beater/last_line_imdb.txt','r+')
last_line = int(f.read())
print(last_line)
