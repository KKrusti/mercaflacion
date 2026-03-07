package enricher

import "strings"

// catalanToSpanish maps normalised Catalan tokens (as produced by normalise)
// to their Spanish equivalents.  The values are already in normalised form
// (lowercase, no accents) so they can be used directly in keyword matching.
//
// The dictionary covers the tokens found in Mercadona Catalunya receipts.
// Entries with an empty-string value indicate tokens that should be dropped
// entirely (i.e. they carry no discriminating information in Spanish).
var catalanToSpanish = map[string]string{
	// ── Dairy ────────────────────────────────────────────────────────────────
	"llet":     "leche",
	"semi":     "semidesnatada",
	"llact":    "", // "s/lact" → sin lactosa; drop
	"lact":     "", // residual from "s/lact" after normalise splits on "/"
	"ous":      "huevos",
	"clara":    "clara",
	"pasteu":   "", // pasteuritzada – drop; not used in Spanish product names
	"iogurt":   "yogur",
	"grec":     "griego",
	"lleuger":  "ligero",
	"lleugera": "ligera",
	"nata":     "nata",
	"batre":    "", // "per batre" = para montar; drop
	"mantega":  "mantequilla",
	"manteg":   "mantequilla", // truncated form
	"llevat":   "levadura",
	"fresc":    "fresco",
	"crema":    "crema",

	// ── Meat & poultry ───────────────────────────────────────────────────────
	"pollastre":     "pollo",
	"pollastr":      "pollo", // truncated
	"pollatre":      "pollo", // variant spelling on receipts
	"pit":           "pechuga",
	"llom":          "lomo",
	"pernil":        "jamon",
	"burger":        "hamburguesa",
	"bovi":          "vacuno",
	"gruixuda":      "gruesa",
	"cuixa":         "muslo",
	"cuixetes":      "muslitos",
	"croquetes":     "croquetas",
	"mandonguilles": "albondigas",
	"mig":           "", // "mig pollastre" = medio pollo; drop modifier
	"rostit":        "asado",
	"gall":          "", // "gall dindi" = pavo; drop
	"dindi":         "pavo",
	"secret":        "secreto",
	"engreix":       "",            // "d'engreixament" = cebado; drop
	"cert":          "",            // "de cert" = de corral; drop
	"bur":           "hamburguesa", // "bur m" = burger meat, truncated
	"burg":          "hamburguesa",
	"espatlla":      "paleta", // shoulder cut
	"garró":         "jarrete",
	"garro":         "jarrete",
	"fetge":         "higado",
	"vacum":         "vacuno",
	"porc":          "cerdo",
	"anec":          "pato",
	"confita":       "confitado",
	"cansalada":     "bacon",
	"viada":         "ahumado", // "cansalada viada" = bacon ahumado
	"embotit":       "embutido",
	"minidauets":    "taquitos",
	"daus":          "tacos",
	"tall":          "loncha",
	"talls":         "", // already implied; drop
	"tallat":        "lonchas",
	"tallada":       "cortada",
	"tallades":      "cortadas",
	"rodanxes":      "rodajas",
	"laminat":       "laminado",

	// ── Fish & seafood ───────────────────────────────────────────────────────
	"tonyina":    "atun",
	"salmo":      "salmon",
	"fumat":      "ahumado",
	"verat":      "caballa",
	"filet":      "filetes",
	"musclo":     "mejillon",
	"musclos":    "mejillones",
	"escabetx":   "escabeche",
	"paqu":       "", // drop
	"gamba":      "gamba",
	"pelada":     "pelada",
	"crua":       "cruda",
	"escopinyes": "berberechos",
	"sard":       "sardina",
	"sardinetes": "sardinillas",
	"sardinilla": "sardinilla",
	"seito":      "anchoa",
	"noruec":     "noruego",
	"pota":       "pota",
	"migas":      "migas",

	// ── Vegetables ───────────────────────────────────────────────────────────
	"carbasso":   "calabacin",
	"pebrot":     "pimiento",
	"ceba":       "cebolla",
	"espinaca":   "espinaca",
	"espinac":    "espinaca",
	"esparrec":   "esparrago",
	"esparrecs":  "esparragos",
	"xampinyo":   "champinon",
	"brots":      "brotes",
	"tendres":    "tiernos",
	"llima":      "lima",
	"verd":       "verde",
	"mitja":      "mediano",
	"mitjana":    "mediana",
	"patata":     "patata",
	"patates":    "patatas",
	"tub":        "", // "ceba tub" = cebolla rama; drop
	"cogombre":   "pepino",
	"cogombres":  "pepinos",
	"cogombrets": "pepinillos",
	"alberginia": "berenjena",
	"alvocat":    "aguacate",
	"carxofa":    "alcachofa",
	"broquil":    "brocoli",
	"pastanaga":  "zanahoria",
	"tomaquet":   "tomate",
	"tomàquet":   "tomate",
	"pebr":       "pimiento", // truncated
	"pebre":      "pimienta",
	"vermell":    "rojo",
	"vermella":   "roja",
	"dolc":       "dulce",
	"dolca":      "dulce",
	"pinyol":     "", // "sense pinyol" = sin hueso; drop
	"pinyols":    "",
	"sense":      "",          // "sense" = sin; drop
	"cong":       "congelado", // "cong zip" = congelado zip
	"col":        "col",
	"arrissada":  "rizada",
	"llisa":      "lisa",
	"canonges":   "canonigos",
	"amanida":    "ensalada",
	"nova":       "", // "nova tendra" = tierna nueva; drop
	"tendra":     "tierna",
	"saltat":     "salteado",
	"graellada":  "a la plancha",
	"verdura":    "verdura",
	"verdures":   "verduras",
	"bolet":      "seta",
	"bolets":     "setas",
	"shiitake":   "shiitake",
	"taronja":    "naranja",
	"cirera":     "cereza",
	"platan":     "platano",
	"maduixa":    "fresa",
	"maduixot":   "fresa", // large strawberry; same product family
	"nabiu":      "arandano",
	"castana":    "castana",
	"castanyes":  "castanas",
	"melo":       "melon",
	"gripau":     "", // "meló de pell de gripau" = galia melon; drop modifier
	"llimona":    "limon",
	"espremuda":  "exprimida",
	"poma":       "manzana",
	"manz":       "manzana",
	"cibulet":    "cebollino",
	"coriandre":  "cilantro",
	"all":        "ajo",
	"alls":       "ajos",
	"adobat":     "aliñado",
	"granulat":   "granulado",
	"sec":        "seco",
	"trossos":    "trozos",
	"tros":       "trozo",
	"plana":      "plana",
	"mongeta":    "judias",
	"pesols":     "guisantes",
	"nyora":      "nora",
	"piquillo":   "piquillo",
	"brandi":     "brandy",

	// ── Legumes & grains ─────────────────────────────────────────────────────
	"cigro":     "garbanzo",
	"cuit":      "cocido",
	"cuita":     "cocida",
	"nyoquis":   "gnocchi",
	"arros":     "arroz",
	"arrós":     "arroz",
	"llentia":   "lenteja",
	"mongetes":  "judias",
	"farina":    "harina",
	"blat":      "trigo",
	"segol":     "centeno",
	"civada":    "avena",
	"flocs":     "copos",
	"moro":      "maiz",
	"crispet":   "palomitas",
	"fideu":     "fideo",
	"corallini": "corallini", // pasta shape; keep
	"pasta":     "pasta",
	"full":      "", // "pasta de full" = hojaldre; drop
	"fullada":   "hojaldre",
	"integrals": "integrales",
	"snacks":    "snacks",
	"snack":     "snack",

	// ── Dairy / cheese ───────────────────────────────────────────────────────
	"formatge":   "queso",
	"formatges":  "queso",
	"provolone":  "provolone",
	"mescla":     "mezcla",
	"ratllat":    "rallado",
	"anyenc":     "añejo",
	"vell":       "viejo", // "formatge vell fort" = queso viejo curado
	"fort":       "curado",
	"blanc":      "blanco",
	"blanca":     "blanca",
	"manchego":   "manchego",
	"gouda":      "gouda",
	"mozzarella": "mozzarella",
	"perles":     "perlas", // "perles mozzarella" = perlas de mozzarella
	"rotlle":     "rulo",   // "rotlle de cabra" = rulo de cabra
	"cabra":      "cabra",
	"curat":      "curado",

	// ── Bread & bakery ───────────────────────────────────────────────────────
	"xapata":       "chapata",
	"torrat":       "tostado",
	"torrada":      "tostada",
	"panses":       "pasas",
	"vidre":        "", // "pa de vidre" = chapata; drop
	"pages":        "campo",
	"pags":         "campo",
	"galeta":       "galleta",
	"maria":        "maria",
	"panet":        "panecillo",
	"llavors":      "semillas",
	"entrepa":      "bocadillo",
	"unitats":      "",            // "5 unitats" = 5 unidades; drop
	"hamb":         "hamburguesa", // "pa hamb" = pan para hamburguesa
	"rustic":       "rustico",
	"rustica":      "rustica",
	"pitas":        "pitas",
	"polvoro":      "polvoron",
	"mini":         "mini",
	"pastissets":   "pastelitos",
	"barreta":      "barrita",
	"barr":         "barrita", // truncated
	"farcida":      "rellena",
	"farcits":      "rellenos",
	"torró":        "turron",
	"torro":        "turron",
	"bombo":        "bombon",
	"ametllat":     "almendrado",
	"xoco":         "chocolate",
	"xocolata":     "chocolate",
	"napo":         "napolitana",
	"fondre":       "fundir", // "xoco fondre" = chocolate para fundir
	"banoffee":     "banoffee",
	"cheesecake":   "cheesecake",
	"festuc":       "pistacho",
	"avellana":     "avellana",
	"cacauet":      "cacahuete", // Catalan spelling (without h)
	"cacahuet":     "cacahuete", // Mercadona receipt spelling (with h)
	"cacahuete":    "cacahuete",
	"desgreixat":   "desgrasado",
	"caramel":      "caramel",
	"salat":        "salado",
	"cornet":       "cucurucho",
	"gelat":        "helado",
	"orxata":       "horchata",
	"mousse":       "mousse",
	"postre":       "postre",
	"pastis":       "pastel",
	"llaminadures": "chuches",
	"tramussos":    "chochos",
	"popitos":      "palomitas",

	// ── Oils & condiments ────────────────────────────────────────────────────
	"oliva":      "aceituna",
	"olives":     "aceitunas",
	"oli":        "aceite",
	"verge":      "virgen",
	"extra":      "extra",
	"arbequina":  "arbequina",
	"negra":      "negra",
	"negre":      "negro",
	"estil":      "estilo",
	"casola":     "casero",
	"orenga":     "oregano",
	"comi":       "comino",
	"safra":      "azafran",
	"bitxo":      "guindilla",
	"molt":       "molido",
	"molta":      "molida",
	"anitines":   "anchoas", // "anitines amb tomàquet" = boquerones con tomate
	"maionesa":   "mayonesa",
	"hummus":     "hummus",
	"truita":     "tortilla",
	"curri":      "curry",
	"enceball":   "cebolleta",
	"sal":        "sal",
	"sucre":      "azucar",
	"llustre":    "glas", // "sucre de llustre" = azucar glas
	"vinagre":    "vinagre",
	"amaniment":  "aliño",
	"bicarbonat": "bicarbonato",
	"alcohol":    "alcohol",

	// ── Drinks ───────────────────────────────────────────────────────────────
	"cervesa":     "cerveza",
	"cerv":        "cerveza", // truncated
	"cerves":      "cerveza",
	"radler":      "radler",
	"llauna":      "lata",
	"ampolla":     "botella",
	"ampolleta":   "botella",
	"agua":        "agua",
	"aigua":       "agua",
	"gas":         "gas",
	"fred":        "frio",
	"micelar":     "micelar",
	"destil":      "destilada",
	"agudes":      "con gas", // "Font Agudes" = agua con gas
	"font":        "fuente",  // "Font Agudes" brand; keep
	"isotonic":    "isotonica",
	"isotonica":   "isotonica",
	"isotònic":    "isotonica",
	"energ":       "energetica",
	"sprite":      "sprite",
	"brou":        "caldo",
	"cafe":        "cafe",
	"descafeinat": "descafeinado",
	"gra":         "grano",
	"nescafe":     "nescafe",

	// ── Snacks & sweets ──────────────────────────────────────────────────────
	"cacau":   "cacao",
	"muesli":  "muesli",
	"fruites": "frutas",
	"crunchy": "crunchy",

	// ── Cleaning & household ─────────────────────────────────────────────────
	"lleixiu":        "lejia",
	"netejador":      "limpiador",
	"neteja":         "limpia",
	"netej":          "limpiador", // truncated
	"netejamaquines": "limpiahogar",
	"netejaulleres":  "quitagrasas",
	"detergent":      "detergente",
	"det":            "detergente", // truncated
	"rentaplats":     "lavavajillas",
	"rentavaixelles": "lavavajillas",
	"suavitzant":     "suavizante",
	"desgreixador":   "desengrasante",
	"desgreix":       "desengrasante",
	"marxe":          "", // brand name; drop
	"fregasols":      "friegasuelos",
	"impulsor":       "quitamanchas",
	"floral":         "floral",
	"marsella":       "marsella",
	"amoniac":        "amoniaco",
	"citric":         "citrico",
	"desinfectant":   "desinfectante",
	"teixits":        "tejidos",
	"vidres":         "cristales",
	"multiusos":      "multiusos",
	"vitroceram":     "vitroceramica",
	"anticalc":       "antical",
	"forn":           "horno",
	"wc":             "wc",
	"banys":          "banos",
	"pistola":        "pistola",
	"espra":          "spray",
	"gel":            "gel",
	"escumós":        "espumoso",
	"escumos":        "espumoso",

	// ── Personal care ────────────────────────────────────────────────────────
	"xampu":       "champu",
	"champu":      "champu",
	"conditioner": "acondicionador",
	"rep":         "", // "rep&nutr" = repair & nutrition; drop
	"nutr":        "", // drop
	"color":       "color",
	"intense":     "intenso",
	"argan":       "argan",
	"deo":         "desodorante",
	"body":        "body",
	"aqua":        "aqua",
	"limit":       "limit",
	"limite":      "limite",
	"power":       "power",
	"rexona":      "rexona",
	"antitaques":  "antimanchas",
	"vaselina":    "vaselina",
	"hidratant":   "hidratante",
	"labial":      "labial",
	"protect":     "protector",
	"fps15":       "fps15",
	"serum":       "serum",
	"hialuronic":  "hialuronico",
	"vitamina":    "vitamina",
	"tonic":       "tonico",
	"depil":       "depilatorio",
	"fulles":      "hojas", // "2 fulles" = 2 hojas
	"ulls":        "ojos",
	"perf":        "perfilador", // "perf. ulls" = perfilador de ojos
	"water":       "waterproof",
	"aut":         "", // "aut" = automatic; drop
	"discs":       "discos",
	"desmaquill":  "desmaquillante",
	"rod":         "", // "discs rodons" = discos redondos; drop
	"raspall":     "cepillo",
	"electric":    "electrico",
	"rec":         "recambio", // "rec. raspall" = recambio cepillo
	"tampo":       "tampon",
	"tamp":        "tampon",
	"compact":     "compacto",
	"regular":     "regular",
	"salva":       "salvaslip",
	"salvaungles": "quitaesmalte",
	"ungles":      "unas",
	"delicat":     "delicado",
	"pinca":       "pinza",
	"obliqua":     "oblicua",
	"agulles":     "agujas",
	"plast":       "plastico",
	"punta":       "punta",

	// ── Baby & household ─────────────────────────────────────────────────────
	"tovall":    "toallita",
	"tovallons": "servilletas",
	"mocador":   "panuelo",
	"mocadors":  "panuelos",
	"capsa":     "caja",
	"capes":     "capas",
	"bebe":      "bebe",
	"fres":      "frescos",

	// ── Kitchen & packaging ──────────────────────────────────────────────────
	"film":          "film",
	"transparent":   "transparente",
	"alumini":       "aluminio",
	"paper":         "papel",
	"higienic":      "higienico",
	"vegetal":       "vegetal",
	"motlle":        "molde",
	"rodo":          "redondo",
	"bossa":         "bolsa",
	"bosses":        "bolsas",
	"zip":           "zip",
	"basura":        "basura",
	"compost":       "compostable",
	"facil":         "facil",
	"tanca":         "cierre",
	"paperera":      "papelera",
	"palla":         "paja",
	"reutilitzable": "reutilizable",
	"encenedor":     "encendedor",
	"fusta":         "madera",
	"grans":         "grandes",
	"llumins":       "cerillas",
	"espelma":       "vela",
	"num":           "numero",
	"carbó":         "carbon",
	"carbo":         "carbon",
	"pal":           "palo",
	"antilliscant":  "antideslizante",
	"sorra":         "arena",
	"aglomerant":    "aglomerante",
	"escombra":      "escoba",
	"interiors":     "interiores",
	"recollidor":    "recogedor",
	"manec":         "mango",
	"rodet":         "rodillo",
	"polsim":        "quitapolvos",
	"adhe":          "adhesivo",
	"tiràs":         "tiras",
	"tiras":         "tiras",
	"atrapapols":    "atrapa polvo",
	"camussa":       "gamuza",
	"baieta":        "bayeta",
	"baietes":       "bayetas",
	"microf":        "microfibra",
	"micro":         "microfibra",
	"fregall":       "estropajo",
	"lot":           "pack", // "lot 3 baietes" = pack 3 bayetas
	"llar":          "",     // "rotlle llar" = rollo hogar; drop
	"doble":         "doble",
	"maquines":      "maquinas",
	"liquid":        "liquido",
	"liquida":       "liquida",
	"recanvi":       "recambio",
	"love":          "", // brand; drop
	"insecticida":   "insecticida",
	"insec":         "insecticida",
	"espirals":      "espirales",
	"mosquits":      "mosquitos",
	"repellent":     "repelente",
	"antihumitat":   "antihumedad",
	"joc":           "pack", // "joc antihumitat" = pack antihumedad

	// ── Health & safety ──────────────────────────────────────────────────────
	"guants":     "guantes",
	"guant":      "guante",
	"nitril":     "nitrilo",
	"nitrils":    "nitrilo",
	"latex":      "latex",
	"mascareta":  "mascarilla",
	"mascaretes": "mascarillas",
	"proteccio":  "proteccion",

	// ── Pet care ─────────────────────────────────────────────────────────────
	"gat":      "gato",
	"silice":   "silice",
	"mixtura":  "mezcla",
	"can":      "perro", // "can/caderner" = perros/canarios
	"caderner": "canario",

	// ── Misc (drop tokens) ───────────────────────────────────────────────────
	"amb":         "", // "amb" = con (preposition); no value for matching
	"nat":         "", // abbreviation; drop
	"granel":      "", // "a granel" = bulk; drop
	"fam":         "", // abbreviation; drop
	"trev":        "", // brand suffix; drop
	"white":       "white",
	"ultra":       "ultra",
	"light":       "light",
	"classic":     "clasico",
	"trico":       "tricolor",
	"baby":        "baby",
	"extrafinatr": "extrafino",
	"net":         "",
}

// translateCatalan replaces Catalan tokens in a normalised product name with
// their Spanish equivalents.  Tokens not present in the dictionary are kept
// unchanged.  Empty-string values cause the token to be dropped.
//
// The input must already be normalised (output of normalise).
func translateCatalan(normalised string) string {
	tokens := strings.Fields(normalised)
	out := make([]string, 0, len(tokens))
	for _, tok := range tokens {
		if replacement, found := catalanToSpanish[tok]; found {
			if replacement != "" {
				out = append(out, replacement)
			}
			// empty replacement → drop token
		} else {
			out = append(out, tok)
		}
	}
	return strings.Join(out, " ")
}
