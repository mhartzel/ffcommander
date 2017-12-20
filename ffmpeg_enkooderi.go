package main

import (
	"fmt"
	"os/exec"
	"strings"
	"flag"
	"log"
	"strconv"
)

// Global map, slice, variable definitions
// var complete_stream_info_map = make(map[int][]string)
var complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)


func run_external_command(command_to_run_str_slice []string) ([]string,  error) {

	var unsorted_ffprobe_information_str_slice []string
	command_output_str := ""

	// Create the struct needed for running the external command
	command_struct := exec.Command(command_to_run_str_slice[0], command_to_run_str_slice[1:]...)

	// Run external command
	command_output, error_message := command_struct.CombinedOutput()

	command_output_str = string(command_output) // The output of the command is []byte convert it to a string

	// Split the output of the command to lines and store in a slice
	for _, line := range strings.Split(command_output_str, "\n")  {
		unsorted_ffprobe_information_str_slice = append(unsorted_ffprobe_information_str_slice, line)
	}

	return unsorted_ffprobe_information_str_slice, error_message
}

func sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice []string) () {

	// Parse ffprobe output, find wrapper, video- and audiostream information in it,
        // and store this info in stream specific maps
        // Store all stream maps in slices. FIXME päteeks tää lause ?

	// var complete_stream_info_map = make(map[int][]string)
	// var video_stream_info_map = make(map[string]string)
	// var audio_stream_info_map = make(map[string]string)
	// var wrapper_info_map = make(map[string]string)

	var stream_info_str_slice []string
	var text_line_str_slice []string
	var wrapper_info_str_slice []string

	var stream_number_int int
	var stream_data_str string
	var string_to_remove_str_slice []string

	// Collect information about all streams in the media file.
        // The info is collected to stream specific slices and stored in map: complete_stream_info_map
        // The stream number is used as the map key when saving info slice to map

	for _,text_line := range unsorted_ffprobe_information_str_slice {
		stream_number_int = -1
		stream_info_str_slice = nil

		// If there are many programs in the file, then the stream information is listed twice by ffprobe,
                // discard duplicate data.
		if strings.HasPrefix(text_line, "programs.program"){
			continue
		}

		if strings.HasPrefix(text_line, "streams.stream") {
			text_line_str_slice = strings.Split(strings.Replace(text_line, "streams.stream.","",1),".")

			// Convert stream number from string to int
			if _, error := strconv.Atoi(text_line_str_slice[0]) ; error == nil {
				stream_number_int,_ = strconv.Atoi(text_line_str_slice[0])
			} else {
				// Stream number could not be understood, skip the stream
				continue
			}

			// If stream number is -1 then we did not find the stream number, skip the stream
			if stream_number_int < 0 {
				continue
			}

			string_to_remove_str_slice = string_to_remove_str_slice[:0] // Empty the slice so that allocated slice ram space remains and is not garbage collected.
			string_to_remove_str_slice = append(string_to_remove_str_slice, "streams.stream.",strconv.Itoa(stream_number_int),".")
			stream_data_str = strings.Replace(text_line, strings.Join(string_to_remove_str_slice,""),"",1) // Remove the unwanted string in front of the text line.
			stream_data_str = strings.Replace(stream_data_str, "\"", "", -1) // Remove " characters from the data.

			// Add found stream info line to a slice of previously stored info
			// and store it in a map. The stream number acts as the map key.
			if _, item_found := complete_stream_info_map[stream_number_int] ; item_found == true {
				stream_info_str_slice = complete_stream_info_map[stream_number_int]
			}
			stream_info_str_slice = append(stream_info_str_slice, stream_data_str)
			complete_stream_info_map[stream_number_int] = stream_info_str_slice
		}
		// Get media file wrapper information and store it in a slice.
		if strings.HasPrefix(text_line, "format") {
			wrapper_info_str_slice = strings.Split(strings.Replace(text_line, "format.", "", 1), "=")
			wrapper_info_map[strings.TrimSpace(wrapper_info_str_slice[0])] = strings.TrimSpace(strings.Replace(wrapper_info_str_slice[1],"\"", "", -1))
		}

		// Find video and audio stream information and store it as key value pairs in video and audio specific maps.
		// Discard streams that are not audio or video
		var stream_type_is_video bool = false
		var stream_type_is_audio bool = false
		text_line = ""

		for _, stream_info_str_slice := range complete_stream_info_map {

			stream_type_is_video = false
			stream_type_is_audio = false

			for _, text_line := range stream_info_str_slice {

				if strings.Contains(text_line, "codec_type=video") {
					stream_type_is_video = true
				}

			}

			for _, text_line := range stream_info_str_slice {

				if strings.Contains(text_line, "codec_type=audio") {
					stream_type_is_audio = true
				}

			}

			if stream_type_is_video == true {

				for _, text_line := range stream_info_str_slice {

					temp_slice := strings.Split(text_line, "=")
					video_key := strings.TrimSpace(temp_slice[0])
					video_value := strings.TrimSpace(temp_slice[1])
					video_stream_info_map[video_key] = video_value
					}

			}

			if stream_type_is_audio == true {
				for _, text_line := range stream_info_str_slice {

					temp_slice := strings.Split(text_line, "=")
					audio_key := strings.TrimSpace(temp_slice[0])
					audio_value := strings.TrimSpace(temp_slice[1])
					audio_stream_info_map[audio_key] = audio_value
					}

			}
		}

	}
}


// FIXME Nyt kaikkien videoiden tiedot summautuu slaisseihin, kun slaisseja ei nollata missään.
// Tässä koodissa on ilmeisesti vielä sekaisin globaalit ja paikalliset slaissit.
// Lisäks pitää funtsata miten lopulliset striimitiedot tallennetaan: Ehkä näin:
// video_stream_info_map, audio_stream_info_map ja wrapper_info_map tallennetaan listaan: video_file_info_list
// ja tämä puolestaan mäppiin all_video_files_info_map, jossa avaimena on tiedoston nimi.
// Tässä on vielä se ongelma, että audiostreameja voi olla monia. Nyt koodi ottaa vain yhden audiostreamin tiedot
// tai vielä todennäköisemmin yhdistää kaikkien audiostreamien tiedot. Pitää estää muiden kuin ensimmäisen audiostreamin käsittely.
// Olisko tämä nyt struct:in paikka ?

func main() {

	// Define commandline options
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace")
	var subtitle_int = flag.Int("s", 0, "Subtitle `number`")
	var grayscale_bool = flag.Bool("gr", false, "Grayscale")
	var denoise_bool = flag.Bool("dn", false, "Denoise")
	var force_stereo_bool = flag.Bool("st", false, "Force Audio To Stereo")
	var autocrop_bool = flag.Bool("ac", false, "Autocrop")
	var force_hd_bool = flag.Bool("hd", false, "Force Video To HD, Profile = High, Level = 4.1, Bitrate = 8000k")
	var input_filenames []string

	// Parse commandline options
	flag.Parse()

	// The unparsed options left on the commandline are filenames, store them in a slice.
	for _,file_name := range flag.Args()  {
		input_filenames = append(input_filenames, file_name)
	}

	// star_options_for_the_filter := "-vf "
	// decomb_options_string := "idet,yadif=0:deint=interlaced"
	// denoise_options_string := ",hqdn3d=3.0:3.0:2.0:3.0"

	// FIXME
	fmt.Println(*autocrop_bool, *grayscale_bool, *subtitle_int, *no_deinterlace_bool, *denoise_bool, *force_stereo_bool, *force_hd_bool)
	fmt.Println("\nSlice:", input_filenames)
	fmt.Println("\n")


	for _,file_name := range input_filenames {

		var command_to_run_str_slice []string

		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe","-loglevel","16","-show_entries","format:stream","-print_format","flat","-i")
		command_to_run_str_slice = append(command_to_run_str_slice, file_name)

		unsorted_ffprobe_information_str_slice, error_message := run_external_command(command_to_run_str_slice)

		if error_message != nil {
			log.Fatal(error_message)
		}

		sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice)

		// FIXME
		fmt.Println(file_name, "complete_stream_info_map:", "\n")
		// for item, stream_info_str_slice := range complete_stream_info_map {
		for key, stream_info_str_slice := range complete_stream_info_map {
			fmt.Println("\n")
			fmt.Println("key:", key)
			fmt.Println("-----------------------------------")
			// fmt.Println("stream_info_str_slice:", stream_info_str_slice)
			for _,value := range stream_info_str_slice {
				fmt.Println(value)
			}
			// fmt.Println(item, " = ", complete_stream_info_map[item], "\n")
		}
		fmt.Println("\n")
		fmt.Println("Wrapper info:")
		fmt.Println("-------------")

		for item := range wrapper_info_map {
			fmt.Println(item, "=", wrapper_info_map[item])
		}
		fmt.Println()

		fmt.Println("video_stream_info_map:")
		fmt.Println("-----------------------")

		for item := range video_stream_info_map {
			fmt.Println(item, "=", video_stream_info_map[item])
		}
		fmt.Println()

		fmt.Println("audio_stream_info_map:")
		fmt.Println("-----------------------")

		for item := range audio_stream_info_map {
			fmt.Println(item, "=", audio_stream_info_map[item])
		}
		fmt.Println()

	}
}
