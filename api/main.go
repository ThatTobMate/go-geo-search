package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/olivere/elastic"
)

const esIndex = "restaurants"
const esType = "restaurant"
const mapping = `
	{
		"mappings": {
			"restaurant": {
				"properties": {
					"id": {
						"type": "text"
					},
					"name": {
						"type": "text"
					},
					"url": {
						"type": "text"
					},
					"image_url": {
						"type": "text"
					},
					"address": {
						"type": "text"
					},
					"open": {
						"type": "boolean"
					},
					"rating": {
						"type": "integer"
					},
					"price": {
						"type": "integer"
					},
					"delivery_area": {
						"type": "geo_shape", "tree":"quadtree", "precision": "1m"
					}
				}
			}
		}
	}
`

type DeliveryArea struct {
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
}

type Restaurant struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	URL          string   `json:"url"`
	ImageURL     string   `json:"image_url"`
	Address      string   `json:"address"`
	Open         bool     `json:"open"`
	Tags         []string `json:"tags"`
	FoodTags     []string `json:"food_tags"`
	Price        int      `json:"price"`
	Rating       int      `json:"rating"`
	DeliveryArea `json:"delivery_area"`
}

type Result struct {
	Restaurants []Restaurant
}

var (
	elasticClient *elastic.Client
)

func main() {
	var err error
	for {
		elasticClient, err = elastic.NewClient(
			elastic.SetURL("ELASTICSEARCH_URL"),
			elastic.SetSniff(false),
			elastic.SetHealthcheck(false),
			elastic.SetErrorLog(log.New(os.Stdout, "", 0)),
			elastic.SetInfoLog(log.New(os.Stdout, "", 0)),
			elastic.SetTraceLog(log.New(os.Stdout, "", 0)),
		)

		if err != nil {
			log.Println("Crashing")
			log.Println(err)
			time.Sleep(10 * time.Second)
		} else {
			log.Println("Starting")
			break
		}
	}
	ctx, _ := context.WithTimeout(context.Background(), 1*time.Second)

	exists, err := elasticClient.IndexExists(esIndex).Do(ctx)

	if err != nil {
		log.Println(err)
	}

	if !exists {
		_, err := elasticClient.CreateIndex(esIndex).BodyString(mapping).Do(ctx)
		if err != nil {
			log.Println(err)
		}
	}

	router := gin.Default()

	router.Use(cors.Default())

	router.POST("/restaurants", createRestaurants)
	router.GET("/search", searchRestaurants)

	if err = router.Run(":9000"); err != nil {
		log.Fatal(err)
	}
}

func handleError(c *gin.Context, code int, err string) {
	c.JSON(code, gin.H{
		"error": err,
	})
}

func createRestaurants(c *gin.Context) {
	var restaurants []Restaurant
	if err := c.BindJSON(&restaurants); err != nil {
		log.Println(err)
		handleError(c, http.StatusBadRequest, "Invalid JSON")
		return
	}

	bulk := elasticClient.
		Bulk().
		Index(esIndex).
		Type(esType)

	for _, restaurant := range restaurants {
		r := Restaurant{
			ID:           restaurant.ID,
			Name:         restaurant.Name,
			URL:          restaurant.URL,
			ImageURL:     restaurant.ImageURL,
			Address:      restaurant.Address,
			Open:         restaurant.Open,
			Tags:         restaurant.Tags,
			FoodTags:     restaurant.FoodTags,
			Price:        restaurant.Price,
			Rating:       restaurant.Rating,
			DeliveryArea: restaurant.DeliveryArea,
		}

		bulk.Add(elastic.NewBulkIndexRequest().Id(r.ID).Doc(r))
	}

	if _, err := bulk.Do(c.Request.Context()); err != nil {
		log.Println(err)
		handleError(c, http.StatusBadRequest, "Restaurant creation failed")
		return
	}

	c.Status(http.StatusOK)
}

func searchRestaurants(c *gin.Context) {
	log.Println("SEARCHING")
	searchTerm := c.Query("q")
	lat := c.Query("lat")
	lng := c.Query("lng")

	if lng == "" {
		handleError(c, http.StatusBadRequest, "No location params")
		return
	}

	if lat == "" {
		handleError(c, http.StatusBadRequest, "No location params")
		return
	}

	// The olivere/elasticsearch library doesn't have support for geo_shape queries. Need to use raw query.

	s := fmt.Sprintf(`{
		"bool" : {
			"must" : {
				"multi_match" : { "query": "%[3]s", "fields": ["name", "tags"]}
			},
			"filter" : {
				"geo_shape" : {
					"delivery_area": {
						"shape": {
							"type": "point",
							"coordinates": [%[1]s, %[2]s]
						},
						"relation": "intersects"
					}
				}
			}
		}
	}`, lat, lng, searchTerm)
	query := elastic.RawStringQuery(s)

	log.Println(query)

	result, err := elasticClient.Search().
		Index(esIndex).
		Query(query).
		Do(c.Request.Context())

	if err != nil {
		log.Println(err)
		handleError(c, http.StatusInternalServerError, "Query failed :(")
		return
	}

	restaurants := make([]Restaurant, 0)

	for _, hit := range result.Hits.Hits {
		var restaurant Restaurant
		json.Unmarshal(*hit.Source, &restaurant)
		restaurants = append(restaurants, restaurant)
	}

	response := Result{
		Restaurants: restaurants,
	}

	c.JSON(http.StatusOK, response)
}
