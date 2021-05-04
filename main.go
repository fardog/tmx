// Package tmx implements a parser for the TMX file format used in the
// Tiled Map Editor.
package tmx

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
)

// Bitmasks for tile orientation
const (
	TileFlippedHorizontally = 0x80000000
	TileFlippedVertically   = 0x40000000
	TileFlippedDiagonally   = 0x20000000
	TileFlipped             = TileFlippedHorizontally | TileFlippedVertically | TileFlippedDiagonally
)

// Possible Errors
var (
	ErrUnsupportedEncoding      = errors.New("invalid encoding")
	ErrUnsupportedCompression   = errors.New("unsupported compression type")
	ErrNoSuitableTileSet        = errors.New("no suitable tileset found for tiles")
	ErrPropertyNotFound         = errors.New("no property with a given name was found")
	ErrPropertyWrongType        = errors.New("a property was found, but its type was incorrect")
	ErrPropertyFailedConversion = errors.New("the property failed to convert to the expected type")
)

// ObjectID specifies a unique ID
type ObjectID int32

// GlobalID is a per-map global unique ID used in Layer tile definitions
// (TileGlobalRef). It also encodes how the tile is drawn; if it's mirrored
// across an axis, for instance. Typically, you will not use a GlobalID
// directly; it will be mapped for you by various helper methods on other
// structs.
type GlobalID uint32

// IsFlippedHorizontally returns true if the ID specifies a horizontal flip
func (g GlobalID) IsFlippedHorizontally() bool {
	return g&TileFlippedHorizontally != 0
}

// IsFlippedVertically returns true if the ID specifies a vertical flip
func (g GlobalID) IsFlippedVertically() bool {
	return g&TileFlippedVertically != 0
}

// IsFlippedDiagonally returns true if the ID specifies a diagonal flip
func (g GlobalID) IsFlippedDiagonally() bool {
	return g&TileFlippedDiagonally != 0
}

// TileID returns the TileSet-relative TileID for a given GlobalID
func (g GlobalID) TileID(t *TileSet) TileID {
	return TileID(g.BareID() - uint32(t.FirstGlobalID))
}

// BareID returns the actual integer ID without tile flip information
func (g GlobalID) BareID() uint32 {
	return uint32(g &^ TileFlipped)
}

// TileID is a tile id unique to each TileSet; often called the "local tile ID"
// in the Tiled docs.
type TileID uint32

// Map represents a Tiled map, and is the top-level container for the map data
type Map struct {
	Version         string         `xml:"version,attr"`
	Orientation     string         `xml:"orientation,attr"`
	RenderOrder     string         `xml:"renderorder,attr"`
	Width           int            `xml:"width,attr"`
	Height          int            `xml:"height,attr"`
	TileWidth       int            `xml:"tilewidth,attr"`
	TileHeight      int            `xml:"tileheight,attr"`
	HexSideLength   int            `xml:"hexsidelength,attr"`
	StaggerAxis     rune           `xml:"staggeraxis,attr"`
	StaggerIndex    string         `xml:"staggerindex,attr"`
	BackgroundColor string         `xml:"backgroundcolor,attr"`
	NextObjectID    ObjectID       `xml:"nextobjectid,attr"`
	TileSets        []TileSet      `xml:"tileset"`
	Properties      Properties     `xml:"properties>property"`
	Layers          []Layer        `xml:"-"`
	ObjectGroups    []ObjectGroup  `xml:"-"`
	ImageLayers     []ImageLayer   `xml:"-"`
	LayersAndGroups []LayerOrGroup `xml:",any"`
}

// This is a temporary structure we parse from XML to determine relative order
// of layers and object groups.
type LayerOrGroup struct {
	XMLName  xml.Name
	InnerXML []byte     `xml:",innerxml"`
	Attrs    []xml.Attr `xml:",any,attr"`
}

// LayerWithName retrieves the first Layer matching the provided name. Returns
// `nil` if not found.
func (m *Map) LayerWithName(name string) *Layer {
	for i := range m.Layers {
		if m.Layers[i].Name == name {
			return &m.Layers[i]
		}
	}

	return nil
}

// ObjectGroupWithName retrieves the first ObjectGroup matching the provided
// name. Returns `nil` if not found.
func (m *Map) ObjectGroupWithName(name string) *ObjectGroup {
	for i := range m.ObjectGroups {
		if m.ObjectGroups[i].Name == name {
			return &m.ObjectGroups[i]
		}
	}

	return nil
}

// TileSetWithName retrieves the first TileSet matching the provided name.
// Returns `nil` if not found.
func (m *Map) TileSetWithName(name string) *TileSet {
	for i := range m.TileSets {
		if m.TileSets[i].Name == name {
			return &m.TileSets[i]
		}
	}

	return nil
}

// TileSet is a set of tiles, including the graphics data to be mapped to the
// tiles, and the actual arrangement of tiles.
type TileSet struct {
	FirstGlobalID   GlobalID   `xml:"firstgid,attr"`
	Source          string     `xml:"source,attr"`
	Name            string     `xml:"name,attr"`
	TileWidth       int        `xml:"tilewidth,attr"`
	TileHeight      int        `xml:"tileheight,attr"`
	Spacing         int        `xml:"spacing,attr"`
	Margin          int        `xml:"margin,attr"`
	TileCount       int        `xml:"tilecount,attr"`
	Columns         int        `xml:"columns,attr"`
	Properties      Properties `xml:"properties>property"`
	TileOffset      TileOffset `xml:"tileoffset"`
	ObjectAlignment string     `xml:"objectalignment,attr"`
	Image           Image      `xml:"image"`
	TerrainTypes    []Terrain  `xml:"terraintypes>terrain"`
	Tiles           []Tile     `xml:"tile"`
}

// TileWithID returns a pointer to the Tile with a given TileID; nil if one is
// not found.
func (t *TileSet) TileWithID(id TileID) *Tile {
	for i := range t.Tiles {
		if t.Tiles[i].TileID == id {
			return &t.Tiles[i]
		}
	}

	return nil
}

type byFirstGlobalID []TileSet

func (a byFirstGlobalID) Len() int           { return len(a) }
func (a byFirstGlobalID) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byFirstGlobalID) Less(i, j int) bool { return a[i].FirstGlobalID < a[j].FirstGlobalID }

// TileOffset is used to specify an offset in pixels to be applied when drawing
// a tile from the related TileSet
type TileOffset struct {
	X int `xml:"x,attr"`
	Y int `xml:"y,attr"`
}

// Image represents a graphic asset to be used for a TileSet (or other
// element). While maps created with the Tiled editor may not have the image
// embedded, the format can support it; no additional decoding or loading is
// attempted by this library, but the data will be available in the struct.
type Image struct {
	Format           string   `xml:"format,attr"`
	ObjectID         ObjectID `xml:"id,attr"`
	Source           string   `xml:"source,attr"`
	TransparentColor string   `xml:"trans,attr"`
	Width            int      `xml:"width,attr"`
	Height           int      `xml:"height,attr"`
	Data             Data     `xml:"data"`
}

// Terrain defines a type of terrain and its associated tile ID.
type Terrain struct {
	Name       string     `xml:"name,attr"`
	TileID     TileID     `xml:"tile,attr"`
	Properties Properties `xml:"properties>property"`
}

// Tile represents an individual tile within a TileSet
type Tile struct {
	TileID      TileID      `xml:"id,attr"`
	Probability float32     `xml:"probability,attr"`
	Properties  Properties  `xml:"properties>property"`
	Type        string      `xml:"type,attr"`
	Image       Image       `xml:"image"`
	Animation   []Frame     `xml:"animation>frame"`
	ObjectGroup ObjectGroup `xml:"objectgroup"`

	// Raw TerrainType loaded from XML. Not intended to be used directly; use
	// the methods on this struct to accessed parsed data.
	RawTerrainType string `xml:"terrain,attr"`

	// cache values
	terrainType *TerrainType
}

// TerrainType returns a TerrainType objects from the given Tile
func (t *Tile) TerrainType() (*TerrainType, error) {
	if t.RawTerrainType == "" {
		return &TerrainType{}, nil
	}

	if t.terrainType != nil {
		return t.terrainType, nil
	}

	strs := strings.Split(t.RawTerrainType, ",")

	if l := len(strs); l != 4 {
		return t.terrainType, fmt.Errorf(
			"unexpected terrain type specifier %v; expected 4 values, got %v",
			t.RawTerrainType,
			l,
		)
	}

	tid := make([]TileID, 4)
	for i := 0; i < len(strs); i++ {
		strs[i] = strings.TrimSpace(strs[i])
		id, err := strconv.ParseInt(strings.TrimSpace(strs[i]), 10, 32)
		if err != nil {
			return t.terrainType, err
		}
		tid[i] = TileID(id)
	}

	t.terrainType = &TerrainType{
		TopLeft:     tid[0],
		TopRight:    tid[1],
		BottomLeft:  tid[2],
		BottomRight: tid[3],
	}

	return t.terrainType, nil
}

// TerrainType represents the unique corner tiles used by a particular terrain
type TerrainType struct {
	TopLeft     TileID
	TopRight    TileID
	BottomLeft  TileID
	BottomRight TileID
}

// Frame is a frame specifier in a given Animation
type Frame struct {
	TileID       TileID `xml:"tileid,attr"`
	DurationMsec int    `xml:"duration,attr"`
}

// Layer specifies a layer of a given Map; a Layer contains tile arrangement
// information.
type Layer struct {
	Name       string     `xml:"name,attr"`
	X          int        `xml:"x,attr"`
	Y          int        `xml:"y,attr"`
	Z          int        `xml:"-"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	OffsetX    int        `xml:"offsetx,attr"`
	OffsetY    int        `xml:"offsety,attr"`
	Properties Properties `xml:"properties>property"`

	// Raw Data loaded from XML. Not intended to be used directly; use the
	// methods on this struct to accessed parsed data.
	RawData Data `xml:"data"`

	// cache values
	tileGlobalRefs []TileGlobalRef
	tileDefs       []*TileDef
}

// TileGlobalRefs retrieves tile reference data from the layer, after processing
// the raw tile data
func (l *Layer) TileGlobalRefs() ([]TileGlobalRef, error) {
	// if XML-encoded tile data was found, just return that
	if len(l.RawData.TileGlobalRefs) > 0 {
		return l.RawData.TileGlobalRefs, nil
	}

	// if we have a cached set of decoded tilerefs, return that
	if l.tileGlobalRefs != nil {
		return l.tileGlobalRefs, nil
	}

	// otherwise, we need to get the byte data and figure out what's there
	bytes, err := l.RawData.Bytes()
	if err != nil {
		return nil, err
	}

	var uis []uint32
	switch l.RawData.Encoding {
	case "base64":
		if uis, err = decodeB64LayerData(bytes); err != nil {
			return nil, err
		}
	case "csv":
		if uis, err = decodeCSVLayerData(bytes); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnsupportedEncoding
	}

	var trs []TileGlobalRef
	for _, ui := range uis {
		trs = append(trs, TileGlobalRef{
			GlobalID: GlobalID(ui),
		})
	}

	// cache the result
	l.tileGlobalRefs = trs

	return trs, nil
}

// TileDefs gets the definitions for all the tiles in a given Layer, matched
// with the given TileSets
func (l *Layer) TileDefs(tss []TileSet) (tds []*TileDef, err error) {
	if l.tileDefs != nil {
		return l.tileDefs, nil
	}

	tgrs, err := l.TileGlobalRefs()
	if err != nil {
		return tds, err
	}

	sort.Sort(byFirstGlobalID(tss))

	for _, tgr := range tgrs {
		bid := tgr.GlobalID.BareID()

		if bid == 0 {
			tds = append(tds, &TileDef{Nil: true})
			continue
		}

		var ts *TileSet
		for i := range tss {
			t := &tss[i]
			if bid < uint32(t.FirstGlobalID) {
				break
			}

			ts = t
		}

		// if we never found a tileset, the file is invalid; return an error that
		if ts == nil {
			return tds, fmt.Errorf(
				"no suitable tileset found for tile with global ID %v; the file is invalid",
				tgr.GlobalID,
			)
		}

		id := tgr.GlobalID.TileID(ts)
		tds = append(tds, &TileDef{
			ID:                  id,
			GlobalID:            tgr.GlobalID,
			TileSet:             ts,
			Tile:                ts.TileWithID(id),
			HorizontallyFlipped: tgr.GlobalID.IsFlippedHorizontally(),
			VerticallyFlipped:   tgr.GlobalID.IsFlippedVertically(),
			DiagonallyFlipped:   tgr.GlobalID.IsFlippedDiagonally(),
		})
	}

	l.tileDefs = tds

	return tds, nil
}

// Data represents a payload in a given object; it may be specified in several
// different encodings and compressions, or as a straight datastructure
// containing TileGlobalRefs
type Data struct {
	Encoding       string          `xml:"encoding,attr"`
	Compression    string          `xml:"compression,attr"`
	TileGlobalRefs []TileGlobalRef `xml:"tile"`

	// Raw Data loaded from XML. Not intended to be used directly; use the
	// methods on this struct to accessed parsed data.
	RawBytes []byte `xml:",innerxml"`
}

func (d *Data) decodeB64Data() (data []byte, err error) {
	raw := bytes.TrimSpace(d.RawBytes)
	r := bytes.NewReader(raw)
	dec := base64.NewDecoder(base64.StdEncoding, r)

	var reader io.ReadCloser

	switch d.Compression {
	case "zlib":
		if reader, err = zlib.NewReader(dec); err != nil {
			return
		}
	case "gzip":
		if reader, err = gzip.NewReader(dec); err != nil {
			return
		}
	case "":
		reader = ioutil.NopCloser(dec)
	default:
		return nil, ErrUnsupportedCompression
	}
	defer reader.Close()

	data, err = ioutil.ReadAll(reader)

	return
}

// Bytes returns the byte array in the Data object, after being uncompressed and
// decoded. In the case of a non-encoded payload, returning a zero-length array
// is completely valid.
//
// While you may use this function, it is typically expected to be called by
// other internal functions when generating a tile list. However, it is safe to
// be called by users of this library if desired, so is exported.
func (d *Data) Bytes() ([]byte, error) {
	switch d.Encoding {
	case "base64":
		return d.decodeB64Data()
	case "csv":
		return d.RawBytes, nil
	case "":
		return d.RawBytes, nil
	}

	return nil, ErrUnsupportedEncoding
}

// TileGlobalRef is a reference to a tile GlobalID
type TileGlobalRef struct {
	GlobalID GlobalID `xml:"gid,attr"`
}

// TileDef is a representation of an individual hydrated tile, with all the
// necessary data to render that tile; it's built up off of the tile GlobalIDs,
// to give a layer-local TileID, its properties, and the tileset used to render
// it (as a reference).
type TileDef struct {
	Nil                 bool
	ID                  TileID
	GlobalID            GlobalID
	TileSet             *TileSet
	Tile                *Tile
	HorizontallyFlipped bool
	VerticallyFlipped   bool
	DiagonallyFlipped   bool
}

// ObjectGroup is a group of objects within a Map or tile, used to specify
// sub-objects such as polygons.
type ObjectGroup struct {
	Name       string     `xml:"name,attr"`
	Color      string     `xml:"color,attr"`
	X          int        `xml:"x,attr"`
	Y          int        `xml:"y,attr"`
	Z          int        `xml:"-"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	OffsetX    int        `xml:"offsetx,attr"`
	OffsetY    int        `xml:"offsety,attr"`
	DrawOrder  string     `xml:"draworder,attr"`
	Properties Properties `xml:"properties>property"`
	Objects    Objects    `xml:"object"`
}

// Object is an individual object, such as a Polygon, Polyline, or otherwise.
type Object struct {
	ObjectID   ObjectID   `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	Type       string     `xml:"type,attr"`
	X          float64    `xml:"x,attr"`
	Y          float64    `xml:"y,attr"`
	Width      float64    `xml:"width,attr"`
	Height     float64    `xml:"height,attr"`
	Rotation   int        `xml:"rotation,attr"`
	GlobalID   GlobalID   `xml:"gid,attr"`
	Visible    bool       `xml:"visible,attr"`
	Properties Properties `xml:"properties>property"`
	Polygons   []Poly     `xml:"polygon"`
	Polylines  []Poly     `xml:"polyline"`
	Image      Image      `xml:"image"`

	// Raw Extras loaded from XML. Not intended to be used directly; use the
	// methods on this struct to accessed parsed data.
	RawExtra []Tag `xml:",any"`
}

// Objects is an array of Object
type Objects []Object

// WithName retrieves the first object with a given name, nil if none
func (ol Objects) WithName(name string) *Object {
	for _, o := range ol {
		if o.Name == name {
			return &o
		}
	}

	return nil
}

// Ellipse returns true if the object is an ellipse, else false
func (o *Object) Ellipse() bool {
	for _, e := range o.RawExtra {
		if e.XMLName.Local == "ellipse" {
			return true
		}
	}
	return false
}

// Poly represents a collection of points; used to represent a Polyline or
// a polygon
type Poly struct {
	// Raw Points loaded from XML. Not intended to be used directly; use the
	// methods on this struct to accessed parsed data.
	RawPoints string `xml:"points,attr"`
}

// Points returns a list of points in a Poly
func (p *Poly) Points() (pts []Point, err error) {
	rpts := strings.Split(p.RawPoints, " ")

	for _, rpt := range rpts {
		var x, y int64

		xy := strings.Split(rpt, ",")
		if l := len(xy); l != 2 {
			err = fmt.Errorf(
				"unexpected number of coordinates in point destructure: %v in %v",
				l, rpt,
			)

			return
		}

		x, err = strconv.ParseInt(xy[0], 10, 32)
		if err != nil {
			return
		}
		y, err = strconv.ParseInt(xy[1], 10, 32)
		if err != nil {
			return
		}

		pts = append(pts, Point{int(x), int(y)})
	}

	return
}

// Point is an X, Y coordinate in space
type Point struct {
	X, Y int
}

// ImageLayer is a layer consisting of a single image, such as a background.
type ImageLayer struct {
	Name       string     `xml:"name,attr"`
	OffsetX    int        `xml:"offsetx,attr"`
	OffsetY    int        `xml:"offsety,attr"`
	X          int        `xml:"x,attr"`
	Y          int        `xml:"y,attr"`
	Z          int        `xml:"-"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	Opacity    float32    `xml:"opacity,attr"`
	Visible    bool       `xml:"visible,attr"`
	Properties Properties `xml:"properties>property"`
	Image      Image      `xml:"image"`
}

// Property wraps any number of custom properties, and is used as a child of a
// number of other objects.
type Property struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

// Properties is an array of Property objects
type Properties []Property

// WithName returns the first property in a list with a given name, nil if none
func (pl Properties) WithName(name string) *Property {
	for _, p := range pl {
		if p.Name == name {
			return &p
		}
	}

	return nil
}

// Float returns a value from a given float property
func (pl Properties) Float(name string) (v float64, err error) {
	p := pl.WithName(name)
	if p == nil {
		return v, ErrPropertyNotFound
	}

	if p.Type != "float" {
		return v, ErrPropertyWrongType
	}

	if v, err = strconv.ParseFloat(p.Value, 64); err != nil {
		return v, ErrPropertyFailedConversion
	}

	return
}

// Int returns a value from a given integer property
func (pl Properties) Int(name string) (v int64, err error) {
	p := pl.WithName(name)
	if p == nil {
		return v, ErrPropertyNotFound
	}

	if p.Type != "int" {
		return v, ErrPropertyWrongType
	}

	if v, err = strconv.ParseInt(p.Value, 10, 64); err != nil {
		return v, ErrPropertyFailedConversion
	}

	return
}

// Bool returns a value from a given boolean property
func (pl Properties) Bool(name string) (v bool, err error) {
	p := pl.WithName(name)
	if p == nil {
		return v, ErrPropertyNotFound
	}

	if p.Type != "bool" {
		return v, ErrPropertyWrongType
	}

	return p.Value == "true", nil
}

// Tag represents a bare XML tag; it is used to decode some not-attribute-nor-
// data-having properties of other objects, and is not intended for direct use.
type Tag struct {
	XMLName xml.Name
	Content string `xml:",innerxml"`
}

// Decode takes a reader for an XML file, and returns a new Map decoded from
// that XML.
func Decode(r io.Reader) (*Map, error) {
	d := xml.NewDecoder(r)
	m := new(Map)

	if err := d.Decode(m); err != nil {
		return nil, err
	}

	// Parsing layers.
	z := 0
	for _, layerOrGroup := range m.LayersAndGroups {
		data, err := xml.Marshal(layerOrGroup)
		if err != nil {
			return nil, err
		}
		switch layerOrGroup.XMLName.Local {
		case "layer":
			layer := new(Layer)
			xml.Unmarshal(data, layer)
			layer.Z = z
			m.Layers = append(m.Layers, *layer)
			z++
		case "objectgroup":
			objectGroup := new(ObjectGroup)
			xml.Unmarshal(data, objectGroup)
			objectGroup.Z = z
			m.ObjectGroups = append(m.ObjectGroups, *objectGroup)
			z++
		case "imagelayer":
			imageLayer := new(ImageLayer)
			xml.Unmarshal(data, imageLayer)
			imageLayer.Z = z
			m.ImageLayers = append(m.ImageLayers, *imageLayer)
			z++
		}
	}

	return m, nil
}

// Same as Decode, but for TSX files
func DecodeTileset(r io.Reader) (*TileSet, error) {
	d := xml.NewDecoder(r)
	ts := new(TileSet)

	if err := d.Decode(ts); err != nil {
		return nil, err
	}

	return ts, nil
}
