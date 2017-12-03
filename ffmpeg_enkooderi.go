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
var complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)

func run_external_command(command_to_run_str_slice []string) ([]string,  error) {

	var command_output_str_slice []string
	command_output_str := ""

	// Create the struct needed for running the external command
	command_struct := exec.Command(command_to_run_str_slice[0], command_to_run_str_slice[1:]...)

	// Run external command
	command_output, error_message := command_struct.CombinedOutput()

	command_output_str = string(command_output) // The output of the command is []byte convert it to a string

	// Split the output of the command to lines and store in a slice
	for _, line := range strings.Split(command_output_str, "\n")  {
		command_output_str_slice = append(command_output_str_slice, line)
	}

	return command_output_str_slice, error_message
}

func sort_raw_ffprobe_information(unsorted_ffprobe_information_slice []string) () {

	// Parse ffprobe output, find wrapper, video- and audiostream information in it,
        // and store this info in stream specific maps
        // Store all stream maps in slices.

	// var video_stream_temp_slice []string
	// var audio_stream_temp_slice []string
	var temp_info_slice []string
	var temp_item_slice []string
	var temp_wrapper_slice []string

	var stream_number int
	var stream_data string

	// Collect information about all streams in the media file.
        // The info is collected to stream specific slices and stored in map: complete_stream_info_map
        // The stream number is used as the map key when saving info slice to map

	for _,item := range unsorted_ffprobe_information_slice {
		stream_number = -1
		temp_info_slice = nil

		// If there are many programs in the file, then the stream information is listed twice by ffprobe,
                // discard duplicate data.
		if strings.HasPrefix(item, "programs.program"){
			continue
		}

		if strings.HasPrefix(item, "streams.stream") {
			temp_item_slice = strings.Split(strings.Replace(item, "streams.stream.","",1),".")

			if _, error := strconv.Atoi(temp_item_slice[0]) ; error == nil {
				stream_number,_ = strconv.Atoi(temp_item_slice[0])
			} else {
				// Stream number could not be understood, skip the stream
				continue
			}

			// If stream number is -1 then we did not find the stream number, skip the stream
			if stream_number < 0 {
				continue
			}
			
			temp_str_slice := []string{"streams.stream.",strconv.Itoa(stream_number),"."}
			stream_data = strings.Replace(item, strings.Join(temp_str_slice,""),"",1)

			// Add found stream info line to a slice of previous info
			// and store it in a map. The stream number acts as the map key.
			if _, found := complete_stream_info_map[stream_number] ; found {
				temp_info_slice = complete_stream_info_map[stream_number]
			}
			temp_info_slice = append(temp_info_slice, stream_data)
			complete_stream_info_map[stream_number] = temp_info_slice
		}
		// Get media file wrapper information and store it in a slice.
		if strings.HasPrefix(item, "format") {
			temp_wrapper_slice = strings.Split(strings.Replace(item, "format.", "", 1), "=")
			wrapper_info_map[strings.TrimSpace(temp_wrapper_slice[0])] = strings.TrimSpace(strings.Replace(temp_wrapper_slice[1],"\"", "", -1))
		}

		//temp_info_slice = nil
		//temp_item_slice = nil

		//for stream_number,_ := range complete_stream_info_map {
		//	//temp_info_slice := complete_stream_info_map[stream_number]
		//	fmt.Println(stream_number)

		//	//for item := range temp_info_slice {
		//	//	fmt.Printf("%s",item)
		//	//}
		//}


		for _, info_slice := range complete_stream_info_map {

			stream_type_is_video := false
			stream_type_is_audio := false

			for _, info_string := range info_slice {

				if strings.Contains(info_string, "codec_type=\"video\"") {
					stream_type_is_video = true
				}

			}

			for _, info_string := range info_slice {

				if strings.Contains(info_string, "codec_type=\"audio\"") {
					stream_type_is_audio = true
				}

			}

			if stream_type_is_video == true {

				for _, info_string := range info_slice {

					temp_slice := strings.Split(info_string ,"=")
					video_key := temp_slice[0]
					video_value := strings.Replace(temp_slice[1] ,"\"", "", 2)
					video_stream_info_map[video_key] = video_value
					}

			}

			if stream_type_is_audio == true {
				for _, info_string := range info_slice {

					temp_slice := strings.Split(info_string ,"=")
					audio_key := temp_slice[0]
					audio_value := strings.Replace(temp_slice[1] ,"\"", "", 2)
					audio_stream_info_map[audio_key] = audio_value
					}

			}
			// for _,value := range info_slice {
			// 	fmt.Println("value:", value)
			// }
		}

	}
	// Go through the stream info we collected above in map 'complete_stream_info_map' and
        // find and collect audio and video specific info. Store this info in audio and video specific slices.
        // Discard streams that are not audio or video
}

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

		command_output_str_slice, error_message := run_external_command(command_to_run_str_slice)

		if error_message != nil {
			log.Fatal(error_message)
		}

		sort_raw_ffprobe_information(command_output_str_slice)

		// FIXME
		fmt.Println(file_name, "complete_stream_info_map:", "\n")
		// for item, info_slice := range complete_stream_info_map {
		for key, info_slice := range complete_stream_info_map {
			fmt.Println("\n")
			fmt.Println("key:", key)
			fmt.Println("-----------------------------------")
			// fmt.Println("info_slice:", info_slice)
			for _,value := range info_slice {
				fmt.Println(value)
			}
			// fmt.Println(item, " = ", complete_stream_info_map[item], "\n")
		}
		fmt.Println("\n")
		fmt.Println("Wrapper info:")
		fmt.Println("-------------")

		for item := range wrapper_info_map {
			fmt.Println(item, " = ", wrapper_info_map[item])
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



		// ffprobe -loglevel 16 -show_entries format:stream -print_format flat -i /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/00-valmiit/Fifth_Element-1997.m4v.mp4
		// length := len("# Filename") + len(file_name) + len(" #") + 1
		// fmt.Println(strings.Repeat("#", length))
		// fmt.Println("# Filename", file_name, "#")
		// fmt.Println(strings.Repeat("#", length))
		// for _, line := range(command_output_str_slice) {
		// fmt.Println(line)
		// }

		// fmt.Printf("Tyyppi: %T\n", command_output_str_slice)
		// fmt.Println("Len: ",len(command_output_str_slice))
		// fmt.Println("\n")
		// fmt.Println("command_output_str_slice:", command_output_str_slice)

	}
}

