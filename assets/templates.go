package assets

import _ "embed"

// MIITemplate holds the embedded Excel master template for MII.
//
//go:embed templates/MII_master_template.xlsx
var MIITemplate []byte

// SDDTemplate holds the embedded Excel master template for SDD.
//
//go:embed templates/SDD_master_template.xlsx
var SDDTemplate []byte

// AdidataTemplate holds the embedded Excel master template for Adidata.
//
//go:embed templates/Adidata_master_template.xlsx
var AdidataTemplate []byte

// NTTTemplate holds the embedded Excel master template for NTT.
//
//go:embed templates/NTT_master_template.xlsx
var NTTTemplate []byte

// BeritaAcaraTemplate holds the embedded Word template for Berita Acara.
//
//go:embed templates/Berita_Acara-Template.docx
var BeritaAcaraTemplate []byte

// SignatureFontTTF holds the embedded cursive TrueType font used to render typed signatures.
//
//go:embed templates/GreatVibes-Regular.ttf
var SignatureFontTTF []byte
