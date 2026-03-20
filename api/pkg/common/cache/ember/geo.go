package ember

import (
	"fmt"

	"github.com/huynhanx03/judgify/pkg/common/cache"
	"github.com/huynhanx03/judgify/pkg/datastructs/geo"
	"github.com/huynhanx03/judgify/pkg/datastructs/skiplist"
	"github.com/huynhanx03/judgify/pkg/hash"
)

// GeoAdd adds the specified geospatial items (longitude, latitude, name) to the specified key.
// Data is stored into the key as a sorted set.
func (c *Cache[K, V]) GeoAdd(key K, lon, lat float64, member string) (bool, error) {
	// 1. Encode coordinates to 52-bit geohash
	h, err := geo.Encode(geo.CoordRange, lon, lat, 26) // 26 steps * 2 = 52 bits
	if err != nil {
		return false, err
	}

	score := float64(geo.Align52Bits(*h))

	// 2. Delegate to ZAdd
	added, err := c.ZAdd(key, score, member)
	return added > 0, err
}

// GeoPos returns the positions (longitude, latitude) of all the specified members 
// of the geospatial index represented by the sorted set at key.
func (c *Cache[K, V]) GeoPos(key K, members ...string) ([]*cache.Point, error) {
	results := make([]*cache.Point, len(members))

	for i, m := range members {
		score, err := c.ZScore(key, m)
		if err != nil {
			results[i] = nil // Member doesn't exist
			continue
		}

		// Geohash score is stored as float64, convert back to bits
		hBits := uint64(score)
		lon, lat := geo.DecodeToLongLat(geo.CoordRange, geo.Bits{Step: 26, Bits: hBits})
		
		results[i] = &cache.Point{
			Longitude: lon,
			Latitude:  lat,
		}
	}

	return results, nil
}

// GeoDist returns the distance between two members in the geospatial index.
// Default unit is meters ("m").
func (c *Cache[K, V]) GeoDist(key K, m1, m2 string, unit string) (float64, error) {
	pos, err := c.GeoPos(key, m1, m2)
	if err != nil {
		return 0, err
	}

	if pos[0] == nil || pos[1] == nil {
		return 0, fmt.Errorf("member not found")
	}

	dist := geo.Distance(pos[0].Longitude, pos[0].Latitude, pos[1].Longitude, pos[1].Latitude)

	// Convert unit if necessary
	return convertUnit(dist, unit), nil
}

// GeoSearch finds members within a circular area defined by center (lon, lat) and radius.
func (c *Cache[K, V]) GeoSearch(key K, lon, lat, radius float64, unit string) ([]cache.GeoSearchResult, error) {
	keyStr := hash.ToString(key)

	obj, ok := c.store.get(keyStr)
	if !ok {
		return nil, cache.ErrKeyNotFound
	}

	if obj.Type != TypeSortedSet {
		return nil, cache.ErrWrongType
	}

	zset, ok := obj.Value.(*zsetData)
	if !ok {
		return nil, cache.ErrWrongType
	}

	// 1. Calculate search ranges (9 Cells)
	radiusMeters := convertToMeters(radius, unit)
	ranges := geo.GetSearchRanges(geo.CoordRange, lon, lat, radiusMeters)

	var candidates []skiplist.Node
	for _, r := range ranges {
		nodes := zset.sl.RangeByScore(skiplist.ScoreRange{
			Min: float64(r[0]),
			Max: float64(r[1]),
		})
		for _, n := range nodes {
			candidates = append(candidates, *n)
		}
	}

	// 2. Filter and calculate exact distance
	results := make([]cache.GeoSearchResult, 0)
	for _, cand := range candidates {
		// Use DecodeToLongLat on the score (which is aligned 52-bit geohash)
		cLon, cLat := geo.DecodeToLongLat(geo.CoordRange, geo.Bits{Step: 26, Bits: uint64(cand.Score)})
		
		d := geo.Distance(lon, lat, cLon, cLat)
		if d <= radiusMeters {
			results = append(results, cache.GeoSearchResult{
				Member:   cand.Key,
				Distance: convertUnit(d, unit),
				Point: &cache.Point{
					Longitude: cLon,
					Latitude:  cLat,
				},
			})
		}
	}

	obj.Touch(c.now())
	return results, nil
}

func convertToMeters(radius float64, unit string) float64 {
	switch unit {
	case "km":
		return radius * 1000.0
	case "mi":
		return radius * 1609.34
	case "ft":
		return radius / 3.28084
	default:
		return radius
	}
}

func convertUnit(meters float64, unit string) float64 {
	switch unit {
	case "km":
		return meters / 1000.0
	case "mi":
		return meters / 1609.34
	case "ft":
		return meters * 3.28084
	default:
		return meters
	}
}
