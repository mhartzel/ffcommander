package main

import (
	"fmt"
	"os/exec"
	"os"
	"strings"
	"flag"
	"log"
	"strconv"
	"path/filepath"
)

// Global variable definitions
var complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)

var Complete_file_info_slice [][][][]string

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

func sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice []string) {

	// Parse ffprobe output, find wrapper, video- and audiostream information in it,
	// and store this info in global maps: complete_stream_info_map and 

	var stream_info_str_slice []string
	var text_line_str_slice []string
	var stream_number_int int
	var stream_data_str string
	var string_to_remove_str_slice []string
	var error error // a variable named error of type error

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
			error = nil

			if stream_number_int, error = strconv.Atoi(text_line_str_slice[0]) ; error != nil {
				// Stream number could not be understood, skip the stream
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
	}
}

func get_video_and_audio_stream_information(file_name string) {

	// Find video and audio stream information and store it as key value pairs in video_stream_info_map and audio_stream_info_map.
	// Discard info about streams that are not audio or video
	var stream_info_slice [][][]string
	var single_video_stream_info_slice []string
	var all_video_streams_info_slice [][]string
	var single_audio_stream_info_slice []string
	var all_audio_streams_info_slice [][]string
	var single_subtitle_stream_info_slice []string
	var all_subtitle_streams_info_slice [][]string
	var stream_type_is_video bool = false
	var stream_type_is_audio bool = false
	var stream_type_is_subtitle = false

	for _, stream_info_str_slice := range complete_stream_info_map {

		stream_type_is_video = false
		stream_type_is_audio = false
		stream_type_is_subtitle = false
		single_video_stream_info_slice = nil
		single_audio_stream_info_slice = nil
		single_subtitle_stream_info_slice = nil

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

		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=subtitle") {
				stream_type_is_subtitle = true
			}
		}

		if stream_type_is_video == true {

			for _, text_line := range stream_info_str_slice {

				temp_slice := strings.Split(text_line, "=")
				video_key := strings.TrimSpace(temp_slice[0])
				video_value := strings.TrimSpace(temp_slice[1])
				video_stream_info_map[video_key] = video_value
				}

			single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, video_stream_info_map["width"], video_stream_info_map["height"])
			all_video_streams_info_slice = append(all_video_streams_info_slice, single_video_stream_info_slice)
		}

		if stream_type_is_audio == true {

			for _, text_line := range stream_info_str_slice {

				temp_slice := strings.Split(text_line, "=")
				audio_key := strings.TrimSpace(temp_slice[0])
				audio_value := strings.TrimSpace(temp_slice[1])
				audio_stream_info_map[audio_key] = audio_value
			}

			single_audio_stream_info_slice = append(single_audio_stream_info_slice, audio_stream_info_map["tags.language"])
			single_audio_stream_info_slice = append(single_audio_stream_info_slice, audio_stream_info_map["disposition.visual_impaired"])
			single_audio_stream_info_slice = append(single_audio_stream_info_slice, audio_stream_info_map["channels"])
			all_audio_streams_info_slice = append(all_audio_streams_info_slice, single_audio_stream_info_slice)
		}

		if stream_type_is_subtitle == true {

			for _, text_line := range stream_info_str_slice {

				temp_slice := strings.Split(text_line, "=")
				subtitle_key := strings.TrimSpace(temp_slice[0])
				subtitle_value := strings.TrimSpace(temp_slice[1])
				subtitle_stream_info_map[subtitle_key] = subtitle_value

			}

			single_subtitle_stream_info_slice = append(single_subtitle_stream_info_slice, subtitle_stream_info_map["tags.language"])
			single_subtitle_stream_info_slice = append(single_subtitle_stream_info_slice, subtitle_stream_info_map["disposition.hearing_impaired"])
			single_subtitle_stream_info_slice = append(single_subtitle_stream_info_slice, subtitle_stream_info_map["codec_name"])
			all_subtitle_streams_info_slice = append(all_subtitle_streams_info_slice, single_subtitle_stream_info_slice)
		}
	}

	if len(all_video_streams_info_slice) == 0 {
		single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, "0", "0")
		all_video_streams_info_slice = append(all_video_streams_info_slice, single_video_stream_info_slice)
	}

	stream_info_slice = append(stream_info_slice, all_video_streams_info_slice, all_audio_streams_info_slice, all_subtitle_streams_info_slice)
	Complete_file_info_slice = append(Complete_file_info_slice, stream_info_slice)
	complete_stream_info_map = make(map[int][]string) // Clear out stream info map by creating a new one with the same name.

	// FIXME valitse tähän parempi failiesimerkki barbarella.ts
	// Complete_file_info_slice contains one slice for each input file. This file contains slices woth further stream information.
	//
	// Example: [[[/mounttipiste/Sailytettava_Tavara-Digiboxi/00-Yhteinen_Kotidirri/security_now/barbarella.ts 720 576]] [[eng 0 2] [dut 1 2]] [[fin 0 dvb_subtitle] [fin 0 dvb_teletext]]]
	// The first slice contains input file path (/home/mika/Downloads/sn0652.mp3) and video width and height in pixels (600, 600). This slice is packed inside another redundant slice since all slices stored in Complete_file_info_slice must be slices of slices of strings [][]string.
	// 
	// The second slice contains separate slices for all audio streams in the file. The first stream info is stored as slice 0 the next as 1, etc. In this case there is only one audio stream with no languge information (language = ""), the stream is not meant for visually impared (0 = no, 1 = yes), and the audio stream has 1 channels.
	// 
	// The third slice contains separate slices for all subtitle streams in the file. The first stream info is stored as slice 0 the next as 1, etc. In this case there is no subtitle streams.
	// 
	// Data in Complete_file_info_slice for ONE input file contains the following:
	// [ [slice for video information], [slice for audio information], [slice for subtitle information] ]
	// 
	// [ [filename, video width, video height], [audio language, for visually impared, number of audio channels], [subtitle language, fot hearing impared, codec name] ]
	// 
	// 

	return
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	/////////////////////////////////////////////////////
	// Test if ffmpeg and ffprobe can be found in path //
	/////////////////////////////////////////////////////
	if _,error := exec.LookPath("ffmpeg") ; error != nil {
		fmt.Println()
		fmt.Println("Error, cant find FFmpeg in path, can't continue.")
		fmt.Println()
		os.Exit(1)
	}

	if _,error := exec.LookPath("ffprobe") ; error != nil {
		fmt.Println()
		fmt.Println("Error, cant find FFprobe in path, can't continue.")
		fmt.Println()
		os.Exit(1)
	}

	//////////////////////////////////////////
	// Define and parse commandline options //
	//////////////////////////////////////////
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace. By default deinterlace is always used. This option disables it.")
	var subtitle_int = flag.Int("s", -1, "Subtitle `number, -s=1` (Use subtitle number 1 in the source file)")
	var subtitle_offset_int = flag.Int("so", 0, "Subtitle `offset`, -so=55 (move subtitle 55 pixels down), -so=-55 (move subtitle 55 pixels up)")
	var subtitle_downscale = flag.Bool("sd", false, "Subtitle `downscale`. When cropping video widthwise, scale down subtitle to fit on top of the cropped video instead for cropping subtitle. This option results in smaller subtitle font.")
	var audio_stream_number_int = flag.Int("a", 0, "Audio stream number, -a=1 (Use audio stream number 1 in the source file)")
	var grayscale_bool = flag.Bool("gr", false, "Convert video to Grayscale")
	var denoise_bool = flag.Bool("dn", false, "Denoise. Use HQDN3D - filter to remove noise in the picture. Equal to Hanbrake 'medium' noise reduction settings.")
	var autocrop_bool = flag.Bool("ac", false, "Autocrop. Find crop values automatically by scanning the star of the file (1800 seconds)")
	var force_hd_bool = flag.Bool("hd", false, "Force Video To HD, Profile = High, Level = 4.1, Bitrate = 8000k")
	var scan_mode_only_bool = flag.Bool("scan", false, "Only scan inputfile and print video and audio stream info.")
	var debug_mode_on = flag.Bool("debug", false, "Turn on debug mode and show info about internal variables.")
	var search_start_str = flag.String("ss", "", "Seek to position before starting processing. This option is given to FFmpeg as it is. Example -ss 01:02:10 Seek to 1 hour two min and 10 seconds.")
	var processing_time_str = flag.String("t", "", "Duration to process. This option is given to FFmpeg as it is. Example -t 01:02 process 1 min 2 secs of the file.")

	//////////////////////
	// Define variables //
	//////////////////////

	var input_filenames []string
	var deinterlace_options []string
	var grayscale_options []string
	var subtitle_processing_options string
	var ffmpeg_pass_1_commandline []string
	var ffmpeg_pass_2_commandline []string
	var final_crop_string string
	var command_to_run_str_slice []string
	var file_to_process, video_width, video_height string
	var audio_language, for_visually_impared, number_of_audio_channels string
	var subtitle_language, for_hearing_impared, subtitle_codec_name string
	var crop_values_picture_width int
	var crop_values_picture_height int
	var crop_values_width_offset int
	var crop_values_height_offset int
	var unsorted_ffprobe_information_str_slice []string
	var error_message error
	var crop_value_map = make(map[string]int)
	var error_messages []string

	///////////////////////////////
	// Parse commandline options //
	///////////////////////////////
	flag.Parse()

	// The unparsed options left on the commandline are filenames, store them in a slice.
	for _,file_name := range flag.Args()  {

		// Test if input files exist
		if _, err := os.Stat(file_name); os.IsNotExist(err) {

			fmt.Println()
			fmt.Println("Error !!!!!!!")
			fmt.Println("File: " + file_name + " does not exist")
			fmt.Println()

			os.Exit(1)

		} else {
			// Add all existing input file names to a slice
			input_filenames = append(input_filenames, file_name)
		}
	}

	//////////////////////////////////////////////////
	// Define default processing options for FFmpeg //
	//////////////////////////////////////////////////
	video_compression_options_sd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "main", "-level", "4.0", "-b:v", "1600k"}
	video_compression_options_hd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "4.1", "-b:v", "8000k"}
	audio_compression_options := []string{"-acodec", "copy"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}

	if *no_deinterlace_bool == true {
		deinterlace_options = []string{"copy"}
	} else {
		deinterlace_options = []string{"idet,yadif=0:deint=interlaced"}
	}
	ffmpeg_commandline_start := []string{"ffmpeg", "-y", "-loglevel", "8", "-threads", "auto"}
	subtitle_number := *subtitle_int

	if *grayscale_bool == false {

		grayscale_options = []string{""}

	} else {

		if subtitle_number == -1 {
			grayscale_options = []string{"lut=u=128:v=128"}
		}

		if subtitle_number >= 0 {
			grayscale_options = []string{",lut=u=128:v=128"}
		}
	}

	subtitle_options := ""
	output_directory_name := "00-valmiit"
	output_video_format := []string{"-f", "mp4"}

	/////////////////////////////////////////
	// Print variable values in debug mode //
	/////////////////////////////////////////
	if *debug_mode_on == true {
		fmt.Println()
		fmt.Println("video_compression_options_sd:",video_compression_options_sd)
		fmt.Println("video_compression_options_hd:",video_compression_options_hd)
		fmt.Println("audio_compression_options:", audio_compression_options)
		fmt.Println("denoise_options:",denoise_options)
		fmt.Println("deinterlace_options:",deinterlace_options)
		fmt.Println("ffmpeg_commandline_start:",ffmpeg_commandline_start)
		fmt.Println("subtitle_number:",subtitle_number)
		fmt.Println("subtitle_offset_int:",*subtitle_offset_int)
		fmt.Println("*subtitle_downscale:",*subtitle_downscale)
		fmt.Println("*grayscale_bool:", *grayscale_bool)
		fmt.Println("grayscale_options:",grayscale_options)
		fmt.Println("subtitle_options:",subtitle_options)
		fmt.Println("*autocrop_bool:", *autocrop_bool)
		fmt.Println("*subtitle_int:", *subtitle_int)
		fmt.Println("*no_deinterlace_bool:", *no_deinterlace_bool)
		fmt.Println("*denoise_bool:", *denoise_bool)
		fmt.Println("*force_hd_bool:", *force_hd_bool)
		fmt.Println("*audio_stream_number_int:", *audio_stream_number_int)
		fmt.Println("*scan_mode_only_bool", *scan_mode_only_bool)
		fmt.Println("*search_start_str", *search_start_str)
		fmt.Println("*processing_time_str", *processing_time_str)
		fmt.Println("*debug_mode_on", *debug_mode_on)
		fmt.Println()
		fmt.Println("input_filenames:", input_filenames)
}

	///////////////////////////////
	// Scan inputfile properties //
	///////////////////////////////

	for _,file_name := range input_filenames {

		// Get video info with: ffprobe -loglevel 16 -show_entries format:stream -print_format flat -i InputFile
		command_to_run_str_slice = nil

		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe","-loglevel","16","-show_entries","format:stream","-print_format","flat","-i")

		if *debug_mode_on == true {
			fmt.Println()
			fmt.Println("command_to_run_str_slice:", command_to_run_str_slice, file_name)
		}

		command_to_run_str_slice = append(command_to_run_str_slice, file_name)

		unsorted_ffprobe_information_str_slice, error_message = run_external_command(command_to_run_str_slice)

		if error_message != nil {
			log.Fatal(error_message)
		}

		// Sort info about video and audio streams in the file to a map
		sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice)

		// Get specific video and audio stream information
		get_video_and_audio_stream_information(file_name)

	}

	if *debug_mode_on == true {

		fmt.Println()
		fmt.Println("Complete_file_info_slices:")

		for _, temp_slice := range Complete_file_info_slice {
			fmt.Println(temp_slice)
		}
	}

	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Test that all input files have a video stream and that the audio and subtitle streams the user wants does exist //
	/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	for _,file_info_slice := range Complete_file_info_slice {

		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]

		audio_slice := file_info_slice[1]
		subtitle_slice := file_info_slice[2]

		file_name_temp := video_slice[0]
		file_name := filepath.Base(file_name_temp)
		video_width := video_slice[1]
		video_height := video_slice[2]

		if video_width == "0" || video_height == "0" {
			error_messages = append(error_messages, "File: '" + file_name + "' does not have a video stream.")
		}

		if len(audio_slice) - 1 < *audio_stream_number_int {
			error_messages = append(error_messages, "File: '" + file_name + "' does not have an audio stream number: " + strconv.Itoa(*audio_stream_number_int))
		}

		if len(subtitle_slice) - 1 < subtitle_number {
			error_messages = append(error_messages, "File: '" + file_name + "' does not have an subtitle stream number: " + strconv.Itoa(subtitle_number))
		}
	}

	// If there were error messages then we can't process all files that the user gave on the commandline, inform the user and exit.
	if len(error_messages) >0 {

		fmt.Println()
		fmt.Println("Error cannot continue !!!!!!!")
		fmt.Println()

		for _, item := range error_messages {
			fmt.Println(item)
		}

		fmt.Println()
		os.Exit(1)
	}

	//////////////////////
	// Scan - only mode //
	//////////////////////

	// Only scan the input files, display their stream properties and exit.
	if *scan_mode_only_bool == true {

		for _,file_info_slice := range Complete_file_info_slice {
			video_slice_temp := file_info_slice[0]
			video_slice := video_slice_temp[0]
			audio_slice := file_info_slice[1]
			subtitle_slice := file_info_slice[2]

			file_to_process_temp := video_slice[0]
			file_to_process = filepath.Base(file_to_process_temp)
			video_width = video_slice[1]
			video_height = video_slice[2]

			fmt.Println()
			subtitle_text := "File name '" + file_to_process + "'"
			text_length := len(subtitle_text)
			fmt.Println(subtitle_text)
			fmt.Println(strings.Repeat("-", text_length))

			fmt.Println("Video width", video_width, ", Video height", video_height)
			fmt.Println()

			for audio_stream_number, audio_info := range audio_slice {

				audio_language = audio_info[0]
				for_visually_impared = audio_info[1]
				number_of_audio_channels = audio_info[2]

				fmt.Println("Audio stream number:", audio_stream_number, ", language:", audio_language, ", For visually impared:", for_visually_impared, ", Number of channels:", number_of_audio_channels)
			}

			fmt.Println()

			for subtitle_stream_number, subtitle_info := range subtitle_slice {

				subtitle_language = subtitle_info[0]
				for_hearing_impared = subtitle_info[1]
				subtitle_codec_name = subtitle_info[2]

				fmt.Println("Subtitle stream number:", subtitle_stream_number, ", language:", subtitle_language, ", For hearing impared:", for_hearing_impared, ", Codec name:", subtitle_codec_name)
			}

			fmt.Println()
		}

		fmt.Println()
		os.Exit(0)
	}

	/////////////////////////////////////////
	// Main loop that processess all files //
	/////////////////////////////////////////

	for _,file_info_slice := range Complete_file_info_slice {

		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]
		file_name := video_slice[0]
		video_width = video_slice[1]
		video_height = video_slice[2]

		// Create input + output filenames and paths
		inputfile_absolute_path,_ := filepath.Abs(file_name)
		inputfile_path := filepath.Dir(inputfile_absolute_path)
		inputfile_name := filepath.Base(file_name)
		output_filename_extension := filepath.Ext(inputfile_name)
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, output_filename_extension) + ".mp4")

		if *debug_mode_on == true {
			fmt.Println("inputfile_path:", inputfile_path)
			fmt.Println("inputfile_name:", inputfile_name)
			fmt.Println("output_file_absolute_path:", output_file_absolute_path)
			fmt.Println("video_width:", video_width)
			fmt.Println("video_height:", video_height)
		}

		// If output directory does not exist in the input path then create it.
		if _, err := os.Stat(filepath.Join(inputfile_path, output_directory_name)); os.IsNotExist(err) {
			os.Mkdir(filepath.Join(inputfile_path, output_directory_name), 0777)
		}

		/////////////////////////////////////////////////////////////
		// Find out autocrop parameters by scanning the input file //
		/////////////////////////////////////////////////////////////

		if *autocrop_bool == true {

			// Create the FFmpeg commandline to scan for blask areas at the borders of the video.
			command_to_run_str_slice = nil
			command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg")

			if *search_start_str == "" {
				command_to_run_str_slice = append(command_to_run_str_slice, "-t","1800")
			}

			command_to_run_str_slice = append(command_to_run_str_slice, "-i",file_name)

			if *search_start_str != "" {
				command_to_run_str_slice = append(command_to_run_str_slice, "-ss", *search_start_str)
			}

			if *processing_time_str != "" {
				command_to_run_str_slice = append(command_to_run_str_slice, "-t", *processing_time_str)
			}
			command_to_run_str_slice = append(command_to_run_str_slice, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:16:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

			crop_value_counter := 0

			if *debug_mode_on == false {
				fmt.Println()
				fmt.Println("Finding crop values for: " + inputfile_name)
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
				fmt.Println()
			}

			ffmpeg_crop_output, ffmpeg_crop_error := run_external_command(command_to_run_str_slice)

			// FFmpeg collects possible crop values across the first 1800 seconds of the file and outputs a list of how many times each possible crop values exists.
			// Parse the list to find the value that is most frequent, that is the value that can be applied without cropping too musch or too little.
			if ffmpeg_crop_error == nil {

				for _,slice_item := range ffmpeg_crop_output {

					for _,item := range strings.Split(slice_item, "\n") {

						if strings.Contains(item, "crop="){

							crop_value := strings.Split(item, "crop=")[1]

							if _,item_found := crop_value_map[crop_value] ; item_found == true {
								crop_value_counter = crop_value_map[crop_value]
							}
							crop_value_counter = crop_value_counter + 1
							crop_value_map[crop_value] = crop_value_counter
							crop_value_counter = 0
						}
					}
				}
				last_crop_value := 0

				for crop_value := range crop_value_map {

					if crop_value_map[crop_value] > last_crop_value {
						last_crop_value = crop_value_map[crop_value]
						final_crop_string = crop_value
					}
				}

				// Store the crop values we will use in variables.
				crop_values_picture_width,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[0])
				crop_values_picture_height,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[1])
				crop_values_width_offset,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[2])
				crop_values_height_offset,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[3])

				/////////////////////////////////////////
				// Print variable values in debug mode //
				/////////////////////////////////////////
				if *debug_mode_on == true {

					fmt.Println()
					fmt.Println("Crop values are:")

					for crop_value := range crop_value_map {
						fmt.Println(crop_value_map[crop_value], "instances of crop values:", crop_value)
						
					}

					fmt.Println()
					fmt.Println("Most frequent crop value is", final_crop_string)
				}

			} else {
				fmt.Println()
				fmt.Println("Scanning inputfile with FFmpeg resulted in an error:")
				fmt.Println(ffmpeg_crop_error)
				fmt.Println()
				os.Exit(1)
			}

			video_height_int, _  := strconv.Atoi(video_height)
			cropped_height := video_height_int - crop_values_picture_height - crop_values_height_offset

			fmt.Println("Crop", crop_values_height_offset, "pixels from the top and", strconv.Itoa(cropped_height), "from the bottom")

			video_width_int, _  := strconv.Atoi(video_width)
			cropped_width := video_width_int - crop_values_picture_width - crop_values_width_offset

			fmt.Println("Crop", crop_values_width_offset, "pixels from the left and", strconv.Itoa(cropped_width), "from the right")
		}

		/////////////////////////
		// Encode video - mode //
		/////////////////////////

		if *scan_mode_only_bool == false {

			ffmpeg_pass_1_commandline = nil
			ffmpeg_pass_2_commandline = nil

			// Create the start of ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_commandline_start...)

			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", file_name)

			if *search_start_str != "" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", *search_start_str)
			}

			if *processing_time_str != "" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-t", *processing_time_str)
			}

			ffmpeg_filter_options := ""

			//////////////////////////////////////////////////////////////////////////////////////////////
			// If there is no subtitle to process use the simple video processing chain (-vf) in FFmpeg //
			// It has a processing pipleine with only one video input and output                        //
			//////////////////////////////////////////////////////////////////////////////////////////////

			if subtitle_number == -1 {
				// There is no subtitle to process add the "no subtitle" option to FFmpeg commandline.
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-sn")

				// Add deinterlace commands to ffmpeg commandline
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(deinterlace_options, "")

				// Add crop commands to ffmpeg commandline
				if *autocrop_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + "crop=" + final_crop_string
				}

				// Add denoise options to ffmpeg commandline
				if *denoise_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(denoise_options, "")
				}

				// Add grayscale options to ffmpeg commandline
				if *grayscale_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options =  ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(grayscale_options, "")
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:v:0", "-vf", ffmpeg_filter_options)

			} else {
				//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
				// There is a subtitle to process with the video, use the complex video processing chain in FFmpeg (-filer_complex) //
				// It can have several simultaneous video inputs and outputs.                                                       //
				//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

				// Add deinterlace commands to ffmpeg commandline
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(deinterlace_options, "")

				// Add crop commands to ffmpeg commandline
				if *autocrop_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + "crop=" + final_crop_string
				}

				// Add denoise options to ffmpeg commandline
				if *denoise_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(denoise_options, "")
				}

				// Add video filter options to ffmpeg commanline
				subtitle_processing_options = "copy"

				// When cropping video widthwise shrink it to fit on top of the cropped video.
				// This results in smaller subtitle font.
				if *autocrop_bool == true && *subtitle_downscale == true {
					subtitle_processing_options = "scale=" + strconv.Itoa(crop_values_picture_width) + ":" + strconv.Itoa(crop_values_picture_height)
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:s:" + strconv.Itoa(subtitle_number) +
				"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
				"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+" +
				strconv.Itoa(*subtitle_offset_int) + strings.Join(grayscale_options, "") + "[processed_combined_streams]", "-map", "[processed_combined_streams]")
			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compressing options to FFmpeg commandline //
			///////////////////////////////////////////////////////////////////

			// If video horizontal resolution is over 700 pixel choose HD video compression settings
			video_compression_options := video_compression_options_sd

			if *force_hd_bool || video_height > "700" {
				video_compression_options = video_compression_options_hd
			}

			// Add video compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, video_compression_options...)

			// Add audio compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, audio_compression_options...)
			
			// Add audiomapping options on the commanline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(*audio_stream_number_int))

			// Add 2 - pass logfile path to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-passlogfile")
			ffmpeg_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, output_filename_extension))
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_2_pass_logfile_path)
		
			// Add video output format to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_video_format...)

			// Copy ffmpeg pass 2 commandline to ffmpeg pass 1 commandline
			ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, ffmpeg_pass_2_commandline...)

			// Add pass 1/2 info on ffmpeg commandline
			ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "-pass", "1")
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-pass", "2")

			// Add /dev/null output option to ffmpeg pass 1 commandline
			ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "/dev/null")

			// Add outfile path to ffmpeg pass 2 commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_file_absolute_path)

			if *debug_mode_on == true {

				fmt.Println()
				fmt.Println("ffmpeg_pass_1_commandline:", ffmpeg_pass_1_commandline)

			} else {

				fmt.Println("Pass 1 encoding: " + inputfile_name)
			}

			// Run Pass 1 encoding with FFmpeg.
			ffmpeg_pass_1_output_temp, ffmpeg_pass_1_error := run_external_command(ffmpeg_pass_1_commandline)

			if *debug_mode_on == true {

				fmt.Println()

				ffmpeg_pass_1_output := strings.TrimSpace(strings.Join(ffmpeg_pass_1_output_temp, ""))

				if len(ffmpeg_pass_1_output) > 0 {
					fmt.Println(len(ffmpeg_pass_1_output))
					fmt.Println(ffmpeg_pass_1_output)
				}

				if ffmpeg_pass_1_error != nil  {
					fmt.Println(ffmpeg_pass_1_error)
				}
			}

			if *debug_mode_on == true {

				fmt.Println()

				fmt.Println("ffmpeg_pass_2_commandline:", ffmpeg_pass_2_commandline)

			} else {

				fmt.Println("Pass 2 encoding: " + inputfile_name)
			}

			// Run Pass 2 encoding with FFmpeg.
			ffmpeg_pass_2_output_temp, ffmpeg_pass_2_error :=  run_external_command(ffmpeg_pass_2_commandline)

			if *debug_mode_on == true {

				fmt.Println()

				ffmpeg_pass_2_output := strings.TrimSpace(strings.Join(ffmpeg_pass_2_output_temp, ""))

				if len(ffmpeg_pass_2_output) > 0 {
					fmt.Println(ffmpeg_pass_2_output)
				}

				if ffmpeg_pass_2_error != nil  {
					fmt.Println(ffmpeg_pass_2_error)
				}

				fmt.Println()
			}

			/////////////////////////////////////
			// Remove ffmpeg 2 - pass logfiles //
			/////////////////////////////////////

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log")
			}

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log.mbtree"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log.mbtree")
			}
		}


	}
}

// FIXME
// Tsekkaa kommentit ja kirjoittele niitä lisää.
// Tsekkaa kannattaisko alun pari - kolme alirutiinia yhdistää yhdeksi.
// Tee default tuloste järkeväksi: Processing file, Looking for crop values, crop values are: x:x:x:, Pass 1 encoding, Pass 2 encoding, Processing of file xxxxxxx finished.
// Tulosta kuinka monta failia on vielä jonossa ja ehkä niiden nimet.
// Tulosta prosessoinnissa yksittäisen failin ja koko käsittelyn kesto, niin jos joutuu ajamaan uudestaan voi vähän arvioidan kauan homma kestää.
// Vaihda kohdehakemiston nimi: 00-valmiit nimeksi: 00-processed_files
// Tulosta hakemistoon 00-valmiit failikohtainen tiedosto, jossa ffmpegin käsittelykomennot, käsittelyn kestot ja kroppiarvot ? Optio jolla tän saa päälle tai oletuksena päälle ja optio jolla saa pois ?
// Laita ohjelma käynnistämään prosesoinnit omiin threadehinsa ja defaulttina kahden tiedoston samanaikainen käsittely. Lisäksi optio jolla voi valita kuinka monta tiedostoa käsitellään samaan aikaan ?
// Jos kroppausarvot on nolla, poista kroppaysoptiot ffmpegin komentoriviltä ?
// 


