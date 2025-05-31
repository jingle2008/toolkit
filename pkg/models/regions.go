package models

// copy from oci-go-sdk/v65@v65.68.0/common/regions.go
type Region string

const (
	//RegionAPChuncheon1 region Chuncheon
	RegionAPChuncheon1 Region = "ap-chuncheon-1"
	//RegionAPHyderabad1 region Hyderabad
	RegionAPHyderabad1 Region = "ap-hyderabad-1"
	//RegionAPMelbourne1 region Melbourne
	RegionAPMelbourne1 Region = "ap-melbourne-1"
	//RegionAPMumbai1 region Mumbai
	RegionAPMumbai1 Region = "ap-mumbai-1"
	//RegionAPOsaka1 region Osaka
	RegionAPOsaka1 Region = "ap-osaka-1"
	//RegionAPSeoul1 region Seoul
	RegionAPSeoul1 Region = "ap-seoul-1"
	//RegionAPSydney1 region Sydney
	RegionAPSydney1 Region = "ap-sydney-1"
	//RegionAPTokyo1 region Tokyo
	RegionAPTokyo1 Region = "ap-tokyo-1"
	//RegionCAMontreal1 region Montreal
	RegionCAMontreal1 Region = "ca-montreal-1"
	//RegionCAToronto1 region Toronto
	RegionCAToronto1 Region = "ca-toronto-1"
	//RegionEUAmsterdam1 region Amsterdam
	RegionEUAmsterdam1 Region = "eu-amsterdam-1"
	//RegionFRA region Frankfurt
	RegionFRA Region = "eu-frankfurt-1"
	//RegionEUZurich1 region Zurich
	RegionEUZurich1 Region = "eu-zurich-1"
	//RegionMEJeddah1 region Jeddah
	RegionMEJeddah1 Region = "me-jeddah-1"
	//RegionMEDubai1 region Dubai
	RegionMEDubai1 Region = "me-dubai-1"
	//RegionSASaopaulo1 region Saopaulo
	RegionSASaopaulo1 Region = "sa-saopaulo-1"
	//RegionUKCardiff1 region Cardiff
	RegionUKCardiff1 Region = "uk-cardiff-1"
	//RegionLHR region London
	RegionLHR Region = "uk-london-1"
	//RegionIAD region Ashburn
	RegionIAD Region = "us-ashburn-1"
	//RegionPHX region Phoenix
	RegionPHX Region = "us-phoenix-1"
	//RegionSJC1 region Sanjose
	RegionSJC1 Region = "us-sanjose-1"
	//RegionSAVinhedo1 region Vinhedo
	RegionSAVinhedo1 Region = "sa-vinhedo-1"
	//RegionSASantiago1 region Santiago
	RegionSASantiago1 Region = "sa-santiago-1"
	//RegionILJerusalem1 region Jerusalem
	RegionILJerusalem1 Region = "il-jerusalem-1"
	//RegionEUMarseille1 region Marseille
	RegionEUMarseille1 Region = "eu-marseille-1"
	//RegionAPSingapore1 region Singapore
	RegionAPSingapore1 Region = "ap-singapore-1"
	//RegionMEAbudhabi1 region Abudhabi
	RegionMEAbudhabi1 Region = "me-abudhabi-1"
	//RegionEUMilan1 region Milan
	RegionEUMilan1 Region = "eu-milan-1"
	//RegionEUStockholm1 region Stockholm
	RegionEUStockholm1 Region = "eu-stockholm-1"
	//RegionAFJohannesburg1 region Johannesburg
	RegionAFJohannesburg1 Region = "af-johannesburg-1"
	//RegionEUParis1 region Paris
	RegionEUParis1 Region = "eu-paris-1"
	//RegionMXQueretaro1 region Queretaro
	RegionMXQueretaro1 Region = "mx-queretaro-1"
	//RegionEUMadrid1 region Madrid
	RegionEUMadrid1 Region = "eu-madrid-1"
	//RegionUSChicago1 region Chicago
	RegionUSChicago1 Region = "us-chicago-1"
	//RegionMXMonterrey1 region Monterrey
	RegionMXMonterrey1 Region = "mx-monterrey-1"
	//RegionUSSaltlake2 region Saltlake
	RegionUSSaltlake2 Region = "us-saltlake-2"
	//RegionSABogota1 region Bogota
	RegionSABogota1 Region = "sa-bogota-1"
	//RegionSAValparaiso1 region Valparaiso
	RegionSAValparaiso1 Region = "sa-valparaiso-1"
	//RegionUSLangley1 region Langley
	RegionUSLangley1 Region = "us-langley-1"
	//RegionUSLuke1 region Luke
	RegionUSLuke1 Region = "us-luke-1"
	//RegionUSGovAshburn1 gov region Ashburn
	RegionUSGovAshburn1 Region = "us-gov-ashburn-1"
	//RegionUSGovChicago1 gov region Chicago
	RegionUSGovChicago1 Region = "us-gov-chicago-1"
	//RegionUSGovPhoenix1 gov region Phoenix
	RegionUSGovPhoenix1 Region = "us-gov-phoenix-1"
	//RegionUKGovLondon1 gov region London
	RegionUKGovLondon1 Region = "uk-gov-london-1"
	//RegionUKGovCardiff1 gov region Cardiff
	RegionUKGovCardiff1 Region = "uk-gov-cardiff-1"
	//RegionAPChiyoda1 region Chiyoda
	RegionAPChiyoda1 Region = "ap-chiyoda-1"
	//RegionAPIbaraki1 region Ibaraki
	RegionAPIbaraki1 Region = "ap-ibaraki-1"
	//RegionMEDccMuscat1 region Muscat
	RegionMEDccMuscat1 Region = "me-dcc-muscat-1"
	//RegionAPDccCanberra1 region Canberra
	RegionAPDccCanberra1 Region = "ap-dcc-canberra-1"
	//RegionEUDccMilan1 region Milan
	RegionEUDccMilan1 Region = "eu-dcc-milan-1"
	//RegionEUDccMilan2 region Milan
	RegionEUDccMilan2 Region = "eu-dcc-milan-2"
	//RegionEUDccDublin2 region Dublin
	RegionEUDccDublin2 Region = "eu-dcc-dublin-2"
	//RegionEUDccRating2 region Rating
	RegionEUDccRating2 Region = "eu-dcc-rating-2"
	//RegionEUDccRating1 region Rating
	RegionEUDccRating1 Region = "eu-dcc-rating-1"
	//RegionEUDccDublin1 region Dublin
	RegionEUDccDublin1 Region = "eu-dcc-dublin-1"
	//RegionAPDccGazipur1 region Gazipur
	RegionAPDccGazipur1 Region = "ap-dcc-gazipur-1"
	//RegionEUMadrid2 region Madrid
	RegionEUMadrid2 Region = "eu-madrid-2"
	//RegionEUFrankfurt2 region Frankfurt
	RegionEUFrankfurt2 Region = "eu-frankfurt-2"
	//RegionEUJovanovac1 region Jovanovac
	RegionEUJovanovac1 Region = "eu-jovanovac-1"
	//RegionMEDccDoha1 region Doha
	RegionMEDccDoha1 Region = "me-dcc-doha-1"
	//RegionEUDccZurich1 region Zurich
	RegionEUDccZurich1 Region = "eu-dcc-zurich-1"
	//RegionMEAbudhabi3 region Abudhabi
	RegionMEAbudhabi3 Region = "me-abudhabi-3"
	// Tacoma, not part of SDK
	RegionTacoma          Region = "us-tacoma-1"
	RegionAPDccTokyo1     Region = "ap-dcc-tokyo-1"
	RegionUSGovSterling2  Region = "us-gov-sterling-2"
	RegionUSGovFortworth1 Region = "us-gov-fortworth-1"
	RegionUSDccPhoenix1   Region = "us-dcc-phoenix-1"
)

var shortNameRegion = map[string]Region{
	"yny": RegionAPChuncheon1,
	"hyd": RegionAPHyderabad1,
	"mel": RegionAPMelbourne1,
	"bom": RegionAPMumbai1,
	"kix": RegionAPOsaka1,
	"icn": RegionAPSeoul1,
	"syd": RegionAPSydney1,
	"nrt": RegionAPTokyo1,
	"yul": RegionCAMontreal1,
	"yyz": RegionCAToronto1,
	"ams": RegionEUAmsterdam1,
	"fra": RegionFRA,
	"zrh": RegionEUZurich1,
	"jed": RegionMEJeddah1,
	"dxb": RegionMEDubai1,
	"gru": RegionSASaopaulo1,
	"cwl": RegionUKCardiff1,
	"lhr": RegionLHR,
	"iad": RegionIAD,
	"phx": RegionPHX,
	"sjc": RegionSJC1,
	"vcp": RegionSAVinhedo1,
	"scl": RegionSASantiago1,
	"mtz": RegionILJerusalem1,
	"mrs": RegionEUMarseille1,
	"sin": RegionAPSingapore1,
	"auh": RegionMEAbudhabi1,
	"lin": RegionEUMilan1,
	"arn": RegionEUStockholm1,
	"jnb": RegionAFJohannesburg1,
	"cdg": RegionEUParis1,
	"qro": RegionMXQueretaro1,
	"mad": RegionEUMadrid1,
	"ord": RegionUSChicago1,
	"mty": RegionMXMonterrey1,
	"aga": RegionUSSaltlake2,
	"bog": RegionSABogota1,
	"vap": RegionSAValparaiso1,
	"lfi": RegionUSLangley1,
	"luf": RegionUSLuke1,
	"ric": RegionUSGovAshburn1,
	"pia": RegionUSGovChicago1,
	"tus": RegionUSGovPhoenix1,
	"ltn": RegionUKGovLondon1,
	"brs": RegionUKGovCardiff1,
	"nja": RegionAPChiyoda1,
	"ukb": RegionAPIbaraki1,
	"mct": RegionMEDccMuscat1,
	"wga": RegionAPDccCanberra1,
	"bgy": RegionEUDccMilan1,
	"mxp": RegionEUDccMilan2,
	"snn": RegionEUDccDublin2,
	"dtm": RegionEUDccRating2,
	"dus": RegionEUDccRating1,
	"ork": RegionEUDccDublin1,
	"dac": RegionAPDccGazipur1,
	"vll": RegionEUMadrid2,
	"str": RegionEUFrankfurt2,
	"beg": RegionEUJovanovac1,
	"doh": RegionMEDccDoha1,
	"avz": RegionEUDccZurich1,
	"ahu": RegionMEAbudhabi3,
	"tiw": RegionTacoma,
	"tyo": RegionAPDccTokyo1,
	"dca": RegionUSGovSterling2,
	"ftw": RegionUSGovFortworth1,
	"ifp": RegionUSDccPhoenix1,
}

// Code not part of SDK
func (r Region) GetCode() string {
	for k, v := range shortNameRegion {
		if v == r {
			return k
		}
	}

	return "UNKNOWN"
}

func CodeToRegion(code string) Region {
	return shortNameRegion[code]
}
