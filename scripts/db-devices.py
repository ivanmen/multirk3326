import sqlite3

db = sqlite3.connect(r"D:\source\multirk3326\prototype\assets\screens\multirk-db.sqlite")
for row in db.execute("select id, host, name, model, paired from devices"):
    print(row)
