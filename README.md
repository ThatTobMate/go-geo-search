Replace `ELASTICSEARCH_URL` with your aws elasticsearch url.

Run go server

```
go run ./api/main.go
```

Populate Elasticsearch Index with dummy data. Could be the endpoint where new data is sent when updated in Admin to keep the index in sync?

```
http://localhost:9000/restaurants -d @data.json -H "Content-Type: application/json"
```

Query Elasticsearch by coordinates and search term (should yield results)
Currently searches by coordinates, restaurant name and tags.

```
curl -X GET http://localhost:9000/search?lat=-0.143147205847431\&lng=51.5218320326981&q=burger
```

Need to write code to do a default search by location if no search term is passed in.
