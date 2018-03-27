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
	"time"
	"sort"
)

// Global variable definitions
var version_number string = "1.01" // This is the version of this program
var Complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)

// Create a slice for storing all video, audio and subtitle stream infos for each input file.
// There can be many audio and subtitle streams in a file.
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

	// Parse ffprobe output, find video- and audiostream information in it,
	// and store this info in the global map: Complete_stream_info_map

	var stream_info_str_slice []string
	var text_line_str_slice []string
	var stream_number_int int
	var stream_data_str string
	var string_to_remove_str_slice []string
	var error error // a variable named error of type error

	// Collect information about all streams in the media file.
        // The info is collected to stream specific slices and stored in map: Complete_stream_info_map
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

			// Remove the text "streams.stream." from the beginning of each text line
			string_to_remove_str_slice = string_to_remove_str_slice[:0] // Clear the slice so that allocated slice ram space remains and is not garbage collected.
			string_to_remove_str_slice = append(string_to_remove_str_slice, "streams.stream.",strconv.Itoa(stream_number_int),".")
			stream_data_str = strings.Replace(text_line, strings.Join(string_to_remove_str_slice,""),"",1) // Remove the unwanted string in front of the text line.
			stream_data_str = strings.Replace(stream_data_str, "\"", "", -1) // Remove " characters from the data.

			// Add found stream info line to a slice with previously stored info
			// and store it in a map. The stream number acts as the map key.
			if _, item_found := Complete_stream_info_map[stream_number_int] ; item_found == true {
				stream_info_str_slice = Complete_stream_info_map[stream_number_int]
			}
			stream_info_str_slice = append(stream_info_str_slice, stream_data_str)
			Complete_stream_info_map[stream_number_int] = stream_info_str_slice
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
	var stream_info_str_slice []string

	// Find text lines in FFprobe info that indicates if this stream is: video, audio or subtitle
	// and store each stream info in a type specific (video, audio and subtitle) slice that in turn gets stored in a slice containing all video, audio or subtitle specific info.

	// First get dictionary keys and sort them
	var dictionary_keys []int

	for key:= range Complete_stream_info_map {
		dictionary_keys = append(dictionary_keys, key)
	}

	sort.Ints(dictionary_keys)

	for _, dictionary_key := range dictionary_keys {
		stream_info_str_slice = Complete_stream_info_map[dictionary_key]

		stream_type_is_video = false
		stream_type_is_audio = false
		stream_type_is_subtitle = false
		single_video_stream_info_slice = nil
		single_audio_stream_info_slice = nil
		single_subtitle_stream_info_slice = nil

		// Find a line in FFprobe output that indicates this is a video stream
		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=video") {
				stream_type_is_video = true
			}
		}

		// Find a line in FFprobe output that indicates this is a audio stream
		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=audio") {
				stream_type_is_audio = true
			}
		}

		// Find a line in FFprobe output that indicates this is a subtitle stream
		for _, text_line := range stream_info_str_slice {

			if strings.Contains(text_line, "codec_type=subtitle") {
				stream_type_is_subtitle = true
			}
		}

		// Store each video stream info text line in a slice and these slices in a slice that collects info for every video stream in the file.
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

		// Store each audio stream info text line in a slice and these slices in a slice that collects info for every audio stream in the file.
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

		// Store each subtitle stream info text line in a slice and these slices in a slice that collects info for every subtitle stream in the file.
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

	// If the input file does not have any video streams in it, store dummy information about a video stream with with and height set to 0 pixels.
	// This will trigger an error message about the file in the main routine (we can't process a file without video)
	if len(all_video_streams_info_slice) == 0 {
		single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, "0", "0")
		all_video_streams_info_slice = append(all_video_streams_info_slice, single_video_stream_info_slice)
	}

	stream_info_slice = append(stream_info_slice, all_video_streams_info_slice, all_audio_streams_info_slice, all_subtitle_streams_info_slice)
	Complete_file_info_slice = append(Complete_file_info_slice, stream_info_slice)
	Complete_stream_info_map = make(map[int][]string) // Clear out stream info map by creating a new one with the same name. We collect information to this map for one input file and need to clear it between processing files.

	// Complete_file_info_slice contains one slice for each input file.
	//
	// The contents is when info for one file is stored: [ [ [/home/mika/Downloads/dvb_stream.ts 720 576]]  [[eng 0 2]  [dut 1 2]]  [[fin 0 dvb_subtitle]  [fin 0 dvb_teletext] ] ]
	//
	// The file path is: /home/mika/Downloads/dvb_stream.ts
	// Video width is: 720 pixels
	// Video height is: 576 pixels
	// The input file has two audio streams
	// Audio stream 0: language is: english, audio is for for visually impared = 0 (false), there are 2 audio channels in the stream.
	// Audio stream 1: language is: dutch, audio is for visually impared = 1 (true), there are 2 audio channels in the stream.
	// The input file has two subtitle streams
	// Subtitle stream 0: language is: finnish, subtitle is for hearing impared = 0 (false), the subtitle codec is: dvb (bitmap)
	// Subtitle stream 1: language is: finnish, subtitle is for hearing impared = 0 (false), the subtitle codec is: teletext
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
	var show_program_version_short = flag.Bool("v", false,"Show the version of this program")
	var show_program_version_long = flag.Bool("version", false,"Show the version of this program")

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
	var video_height_int int
	var video_bitrate string
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
	var file_counter int
	var file_counter_str string
	var files_to_process_str string
	start_time := time.Now()
	pass_1_start_time := time.Now()
	pass_1_elapsed_time := time.Since(start_time)
	pass_2_start_time := time.Now()
	pass_2_elapsed_time := time.Since(start_time)

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

	if *show_program_version_short == true || *show_program_version_long == true {
		fmt.Println()
		fmt.Println("Version:", version_number)
		fmt.Println()
		fmt.Println("(C) Mikael Hartzell 2018.")
		fmt.Println()
		fmt.Println("FFmpeg version 3 or higher is required to use this program.")
		fmt.Println()
		fmt.Println("This program is distributed under the GNU General Public License, version 3 (GPLv3)")
		fmt.Println("Check the license here: http://www.gnu.org/licenses/gpl.txt")
		fmt.Println("Basically this license gives you full freedom to do what ever you want with this program.")
		fmt.Println("You are free to use, modify, distribute it any way you like.")
		fmt.Println("The only restriction is that if you make derivate works of this program AND distribute those,")
		fmt.Println("the derivate works must also be licensed under GPL 3.")
		fmt.Println()
		fmt.Println("This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY;")
		fmt.Println("without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.")
		fmt.Println("See the GNU General Public License for more details")
		fmt.Println()
		os.Exit(0)
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
	output_directory_name := "00-processed_files"
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

				fmt.Printf("Audio stream number: %d, language: %s, for visually impared: %s, number of channels: %s\n", audio_stream_number, audio_language, for_visually_impared, number_of_audio_channels)
			}

			fmt.Println()

			for subtitle_stream_number, subtitle_info := range subtitle_slice {

				subtitle_language = subtitle_info[0]
				for_hearing_impared = subtitle_info[1]
				subtitle_codec_name = subtitle_info[2]

				fmt.Printf("Subtitle stream number: %d, language: %s, for hearing impared: %s, codec name: %s\n", subtitle_stream_number, subtitle_language, for_hearing_impared, subtitle_codec_name)
			}

			fmt.Println()
		}

		fmt.Println()
		os.Exit(0)
	}

	/////////////////////////////////////////
	// Main loop that processess all files //
	/////////////////////////////////////////

	files_to_process_str = strconv.Itoa(len(Complete_file_info_slice))

	for _,file_info_slice := range Complete_file_info_slice {

		start_time = time.Now()
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

		// Print information about processing
		file_counter = file_counter + 1
		file_counter_str = strconv.Itoa(file_counter)

		fmt.Println("")
		fmt.Println(strings.Repeat("#", 80))
		fmt.Println("")
		fmt.Println("Processing file " + file_counter_str + "/" + files_to_process_str + "  '" + inputfile_name + "'")

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
				fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
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

			video_width_int, _  := strconv.Atoi(video_width)
			cropped_width := video_width_int - crop_values_picture_width - crop_values_width_offset

			fmt.Println("Top:", crop_values_height_offset, ", Bottom:", strconv.Itoa(cropped_height), ", Left:", crop_values_width_offset, ", Right:", strconv.Itoa(cropped_width))
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

			video_height_int,_ = strconv.Atoi(video_height)
			video_bitrate = "1600k"

			if *force_hd_bool || video_height_int > 700 {
				video_compression_options = video_compression_options_hd
				video_bitrate = "8000k"
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
				fmt.Println()
				fmt.Println("Encoding with video bitrate:", video_bitrate)
				fmt.Printf("Pass 1 encoding: " + inputfile_name + " ")
			}

			// Run Pass 1 encoding with FFmpeg.
			pass_1_start_time = time.Now()

			ffmpeg_pass_1_output_temp, ffmpeg_pass_1_error := run_external_command(ffmpeg_pass_1_commandline)

			pass_1_elapsed_time = time.Since(pass_1_start_time)
			fmt.Printf("took %s", pass_1_elapsed_time.Round(time.Millisecond))
			fmt.Println()

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

				pass_2_elapsed_time = time.Since(start_time)
				fmt.Printf("Pass 2 encoding: " + inputfile_name + " ")
			}

			// Run Pass 2 encoding with FFmpeg.
			pass_2_start_time = time.Now()

			ffmpeg_pass_2_output_temp, ffmpeg_pass_2_error :=  run_external_command(ffmpeg_pass_2_commandline)

			pass_2_elapsed_time = time.Since(pass_2_start_time)
			fmt.Printf("took %s", pass_2_elapsed_time.Round(time.Millisecond))
			fmt.Println()

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

			elapsed_time := time.Since(start_time)
			fmt.Printf("Processing took %s", elapsed_time.Round(time.Millisecond))
			fmt.Println()
		}


	}
}

// FIXME
// Tulosta hakemistoon 00-processed_files failikohtainen tiedosto, jossa ffmpegin käsittelykomennot, käsittelyn kestot ja kroppiarvot ? Optio jolla tän saa päälle tai oletuksena päälle ja optio jolla saa pois ?
// Jos kroppausarvot on nolla, poista kroppaysoptiot ffmpegin komentoriviltä ?
// Tee enkoodauksen aikainen FFmpegin tulosteen tsekkaus, joka laskee koodauksen aika-arvion ja prosentit siitä kuinka paljon failia on jo käsitelty (fps ?) Tästä on esimerkkiohjelma muistiinpanoissa, mutta se jumittaa n. 90 sekuntia FFmpeg - enkoodauksen alkamisesta.
// 


