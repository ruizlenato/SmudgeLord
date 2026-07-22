package pinterest

import "regexp"

type Handler struct {
	postID string
}

var (
	pinURLRegex    = regexp.MustCompile(`pinterest\.com/pin/(\d+)`)
	shortLinkRegex = regexp.MustCompile(`pinterest\.com/url_shortener/([A-Za-z0-9_-]+)/redirect`)
	relayDataRegex = regexp.MustCompile(`(?s)__PWS_RELAY_REGISTER_COMPLETED_REQUEST__\("[^"]*",\s*(\{"data":\s*\{.+?\})\)\s*;?\s*</script>`)
)

type RelayResponse struct {
	Data map[string]PinQuery `json:"data"`
}

type PinQuery struct {
	Data PinData `json:"data"`
}

type PinData struct {
	Description         string         `json:"description"`
	Title               string         `json:"title"`
	GridTitle           string         `json:"gridTitle"`
	CloseupUnifiedTitle string         `json:"closeupUnifiedTitle"`
	Videos              *PinVideos     `json:"videos"`
	StoryPinData        *StoryPinData  `json:"storyPinData"`
	CarouselData        *CarouselData  `json:"carouselData"`
	ImagesOrig          *ImageDetails  `json:"images_orig"`
	ImageLargeURL       string         `json:"imageLargeUrl"`
	Pinner              *Pinner        `json:"pinner"`
	NativeCreator       *NativeCreator `json:"nativeCreator"`
}

type PinVideos struct {
	VideoList VideoList `json:"video_list"`
}

type VideoList struct {
	V720P  *VideoVariant `json:"v_720p"`
	VHLSV4 *VideoVariant `json:"v_hlsv4"`
	VHLSV3 *VideoVariant `json:"v_hlsv3"`
	VEXP0  *VideoVariant `json:"v_exp0"`
	VEXP1  *VideoVariant `json:"v_exp1"`
	VEXP2  *VideoVariant `json:"v_exp2"`
	VEXP3  *VideoVariant `json:"v_exp3"`
	VEXP4  *VideoVariant `json:"v_exp4"`
	VEXP5  *VideoVariant `json:"v_exp5"`
	VEXP6  *VideoVariant `json:"v_exp6"`
	VEXP7  *VideoVariant `json:"v_exp7"`
	V360P  *VideoVariant `json:"v_360p"`
	V480P  *VideoVariant `json:"v_480p"`
	V1080P *VideoVariant `json:"v_1080p"`
	V240P  *VideoVariant `json:"v_240p"`
}

type VideoVariant struct {
	URL       string `json:"url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Duration  int    `json:"duration"`
	Thumbnail string `json:"thumbnail"`
}

type StoryPinData struct {
	Pages []StoryPinPage `json:"pages"`
}

type StoryPinPage struct {
	Blocks []StoryPinBlock `json:"blocks"`
}

type StoryPinBlock struct {
	Typename    string          `json:"__typename"`
	VideoDataV2 *VideoDataV2    `json:"videoDataV2"`
	ImageData   *StoryImageData `json:"imageData"`
}

type VideoDataV2 struct {
	VideoList720P   *VideoList720P    `json:"videoList720P"`
	VideoListMobile *VideoListMobile  `json:"videoListMobile"`
	VideoList       *VideoListGeneric `json:"videoList"`
	VHLSV4VideoList *VHLSV4VideoList  `json:"v_hlsv4_video_list"`
}

type VideoList720P struct {
	V720P *VideoVariant `json:"v720P"`
}

type VideoListMobile struct {
	VHLSV3Mobile *VideoVariant `json:"vHLSV3MOBILE"`
}

type VideoListGeneric struct {
	VHLSV3Mobile *VideoVariant `json:"vHLSV3MOBILE"`
}

type VHLSV4VideoList struct {
	VHLSV4 *VideoVariant `json:"vHLSV4"`
}

type StoryImageData struct {
	Images StoryPinImages `json:"images"`
}

type StoryPinImages struct {
	Orig *ImageDetails `json:"orig"`
}

type ImageDetails struct {
	URL string `json:"url"`
}

type CarouselData struct {
	CarouselSlots []CarouselSlot `json:"carousel_slots"`
}

type CarouselSlot struct {
	Videos     *PinVideos    `json:"videos"`
	ImagesOrig *ImageDetails `json:"images_orig"`
}

type Pinner struct {
	Username string `json:"username"`
}

type NativeCreator struct {
	Username string `json:"username"`
}
