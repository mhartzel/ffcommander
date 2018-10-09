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
var version_number string = "1.21" // This is the version of this program
var Complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)

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
	var wrapper_info_str_slice []string

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

		// Get media file wrapper information and store it in a slice.
                if strings.HasPrefix(text_line, "format") {
                        wrapper_info_str_slice = strings.Split(strings.Replace(text_line, "format.", "", 1), "=")
                        wrapper_key := strings.TrimSpace(wrapper_info_str_slice[0])
                        wrapper_value := strings.TrimSpace(strings.Replace(wrapper_info_str_slice[1],"\"", "", -1)) // Remove whitespace and " charcters from the data
                        wrapper_info_map[wrapper_key] = wrapper_value
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
	// and store each stream info in a type specific (video, audio and subtitle) slice that
	// in turn gets stored in a slice containing all video, audio or subtitle specific info.

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

			// Add also duration from wrapper information to the video info.
			single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, video_stream_info_map["width"], video_stream_info_map["height"], wrapper_info_map["duration"])
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
			single_audio_stream_info_slice = append(single_audio_stream_info_slice, audio_stream_info_map["sample_rate"])
			single_audio_stream_info_slice = append(single_audio_stream_info_slice, audio_stream_info_map["codec_name"])
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
	// The contents is when info for one file is stored: [ [ [/home/mika/Downloads/dvb_stream.ts 720 576 64.123411]]  [[eng 0 2 48000 ac3]  [dut 1 2 48000 pcm_s16le]]  [[fin 0 dvb_subtitle]  [fin 0 dvb_teletext] ] ]
	//
	// The file path is: /home/mika/Downloads/dvb_stream.ts
	// Video width is: 720 pixels and height is: 576 pixels and the duration is: 64.123411 seconds.
	// The input file has two audio streams (languages: eng and dut)
	// Audio stream 0: language is: english, audio is for for visually impared = 0 (false), there are 2 audio channels in the stream and sample rate is 48000 and audio codec is ac3.
	// Audio stream 1: language is: dutch, audio is for visually impared = 1 (true), there are 2 audio channels in the stream and sample rate is 48000 and audio codec is pcm_s16le.
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
	// Audio options
	var audio_language_str = flag.String("a", "", "Audio language: -a fin or -a eng or -a ita  Only use option -an or -a not both.")
	var audio_stream_number_int = flag.Int("an", 0, "Audio stream number, -a 1 (Use audio stream number 1 from the source file).")
	var audio_compression_ac3 = flag.Bool("ac3", false, "Compress audio as ac3. Channel count adjusts compression bitrate automatically. Stereo uses 192k and 3 - 6 channels uses 640k bitrate.")

	// Video options
	var autocrop_bool = flag.Bool("ac", false, "Autocrop. Find crop values automatically by doing 10 second spot checks in 10 places for the duration of the file.")
	var denoise_bool = flag.Bool("dn", false, "Denoise. Use HQDN3D - filter to remove noise from the picture. This option is equal to Hanbrakes 'medium' noise reduction settings.")
	var grayscale_bool = flag.Bool("gr", false, "Convert video to Grayscale. Use this option if the original source is black and white. This results more bitrate being available for b/w information and better picture quality.")
	var force_hd_bool = flag.Bool("hd", false, "Force video encoding to use HD bitrate and profile (Profile = High, Level = 4.1, Bitrate = 8000k) By default this program decides video encoding profile and bitrate automatically depending on the vertical resolution of the picture.")
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace. By default deinterlace is always used. This option disables it.")

	// Options that affect both video and audio
	var force_lossless_bool = flag.Bool("ls", false, "Force encoding to use lossless 'utvideo' compression for video and 'flac' compression for audio. This also turns on -fe")

	// Subtitle options
	var subtitle_language_str = flag.String("s", "", "Subtitle language: -s fin or -s eng -s ita  Only use option -sn or -s not both.")
	var subtitle_downscale = flag.Bool("sd", false, "Subtitle `downscale`. When cropping video widthwise, scale down subtitle to fit on top of the cropped video instead of cropping the subtitle. This option results in smaller subtitle font.")
	var subtitle_int = flag.Int("sn", -1, "Subtitle stream `number, -sn 1` Use subtitle number 1 from the source file. Only use option -sn or -s not both.")
	var subtitle_vertical_offset_int = flag.Int("so", 0, "Subtitle `offset`, -so 55 (move subtitle 55 pixels down), -so -55 (move subtitle 55 pixels up).")
	var subtitle_mux_bool = flag.Bool("sm", false, "Mux subtitle into the target file. This only works with dvd, dvb and bluray bitmap based subtitles. If this option is not set then subtitles will be burned into the video.")
	var subtitle_palette = flag.String("palette", "", "Hack dvd subtitle color palette. Option takes 1-16 comma separated hex numbers ranging from 0 to f. Zero = black, f = white, so only shades between black -> gray -> white can be defined. FFmpeg requires 16 hex numbers, so f's are automatically appended to the end of user given numbers. Each dvd uses color mapping differently so you need to try which numbers control the colors you want to change. Usually the first 4 numbers control the colors. Example: -palette f,0,f")

	// Scan options
	var fast_bool = flag.Bool("f", false, "This is the same as using options -fs and -fe at the same time.")
	var fast_encode_bool = flag.Bool("fe", false, "Fast encoding mode. Encode video using 1-pass encoding.")
	var fast_search_bool = flag.Bool("fs", false, "Fast seek mode. When using the -fs option with -ss do not decode video before the point we are trying to locate, but instead try to jump directly to it. This search method might or might not be accurate depending on the file format.")
	var scan_mode_only_bool = flag.Bool("scan", false, "Only scan input file and print video and audio stream info.")
	var search_start_str = flag.String("ss", "", "Seek to time position before starting processing. This option is given to FFmpeg as it is. Example -ss 01:02:10 Seeks to 1 hour two minutes and 10 seconds.")
	var processing_time_str = flag.String("t", "", "Duration of video to process. This option is given to FFmpeg as it is. Example -t 01:02 process 1 minuntes and 2 seconds of the file.")

	// Misc options
	var debug_mode_on = flag.Bool("debug", false, "Turn on debug mode and show info about internal variables and the FFmpeg commandlines used.")
	var use_matroska_container = flag.Bool("mkv", false, "Use matroska (mkv) as the output file wrapper format.")
	var show_program_version_short = flag.Bool("v", false,"Show the version of this program.")
	var show_program_version_long = flag.Bool("version", false,"Show the version of this program.")

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
	var file_to_process, video_width, video_height, video_duration string
	var video_height_int int
	var video_bitrate string
	var audio_language, for_visually_impared, number_of_audio_channels, audio_codec string
	var subtitle_language, for_hearing_impared, subtitle_codec_name string
	var crop_values_picture_width int
	var crop_values_picture_height int
	var crop_values_width_offset int
	var crop_values_height_offset int
	var unsorted_ffprobe_information_str_slice []string
	var error_message error
	var error_messages []string
	var file_counter int
	var file_counter_str string
	var files_to_process_str string
	var subtitle_horizontal_offset_int int
	var subtitle_horizontal_offset_str string

	start_time := time.Now()
	pass_1_start_time := time.Now()
	pass_1_elapsed_time := time.Since(start_time)
	pass_2_start_time := time.Now()
	pass_2_elapsed_time := time.Since(start_time)

	output_directory_name := "00-processed_files"


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

	// Test that user gave a string not a number for options -a and -s
	if _, err := strconv.Atoi(*audio_language_str); err == nil {
		fmt.Println()
		fmt.Println("The option -a requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	if _, err := strconv.Atoi(*subtitle_language_str); err == nil {
		fmt.Println()
		fmt.Println("The option -s requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	// -f option turns on both options -fs and -fe
	if *fast_bool == true {
		*fast_search_bool = true
		*fast_encode_bool = true
	}

	// Always use 1-pass encoding with lossless encoding. Turn on option -fe.
	if *force_lossless_bool == true {
		*fast_encode_bool = true
	}

	// Check dvd palette hacking option string correctness.
	if *subtitle_palette != "" {
		temp_slice := strings.Split(*subtitle_palette, ",")
		*subtitle_palette = ""
		hex_characters := [17]string{ "0","1","2","3","4","5","6","7","8","9","a","b","c","d","e","f" }

		// Test that all characters are valid hex
		for _,character := range temp_slice {

			hex_match_found := false

			if character == "" {
				fmt.Println("")
				fmt.Println("Illegal character: 'empty' in -palette option string. Values must be hex ranging from 0 to f.")
				fmt.Println("")
				os.Exit(0)
			}
			for _,hex_value := range hex_characters {

				if strings.ToLower(character) == hex_value {
					hex_match_found = true
					break
				}
			}

			if hex_match_found == false {
				fmt.Println("")
				fmt.Println("Illegal character:",character ,"in -palette option string. Values must be hex ranging from 0 to f.")
				fmt.Println("")
				os.Exit(0)
			}
		}

		// Test that user gave between 1 to 16 characters
		if len(temp_slice) < 1 {
			fmt.Println("")
			fmt.Println("Too few (",len(temp_slice) , ") hex characters in -palette option string. Please give 1 to 16 characters.")
			fmt.Println("")
			os.Exit(0)
		}

		if len(temp_slice) > 16 {
			fmt.Println("")
			fmt.Println("Too many (",len(temp_slice) , ") hex characters in -palette option string. Please give 1 to 16 characters.")
			fmt.Println("")
			os.Exit(0)
		}

		// Prepare -palette option string for FFmpeg. It requires 16 hex strings where each consists of 6 hex numbers. Of these every 2 numbers control RBG color.
		// The user is limited here to use only shades between black -> gray -> white.
		for counter,character := range temp_slice {

			*subtitle_palette = *subtitle_palette + strings.Repeat(strings.ToLower(character), 6)

			if counter < len(temp_slice) - 1 {
				*subtitle_palette = *subtitle_palette + ","
			}

		}

		if len(temp_slice) < 16 {

			*subtitle_palette = *subtitle_palette + ","

			for counter:= len(temp_slice); counter < 16; counter++ {
				*subtitle_palette = *subtitle_palette + "ffffff"

				if counter < 15 {
					*subtitle_palette = *subtitle_palette + ","
				}
			}
		}
	}

	// Print program version and license info.
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
	video_compression_options_lossless := []string{"-c:v", "utvideo"}
	audio_compression_options := []string{"-acodec", "copy"}
	audio_compression_options_2_channels_ac3 := []string{"-c:a","ac3","-b:a","192k"}
	audio_compression_options_6_channels_ac3 := []string{"-c:a","ac3","-b:a","640k"}
	audio_compression_options_lossless_flac := []string{"-acodec", "flac"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}

	// Determine output file container
	output_video_format := []string{"-f", "mp4"}
	output_filename_extension := ".mp4"

	if *force_lossless_bool == true || *use_matroska_container == true {
		// Use matroska as the output file wrapper format
		output_video_format = nil
		output_video_format = append(output_video_format, "-f", "matroska")
		output_filename_extension = ".mkv"
	}

	if *no_deinterlace_bool == true {
		deinterlace_options = []string{"copy"}
	} else {
		// Deinterlacing options used to be: "idet,yadif=0:deint=interlaced"  which tries to detect
		// if a frame is interlaced and deinterlaces only those that are.
		// If there was a cut where there was lots of movement in the picture then some interlace
		// remained in a couple of frames after the cut.
		deinterlace_options = []string{"idet,yadif=0:deint=all"}
	}
	ffmpeg_commandline_start := []string{"ffmpeg", "-y", "-loglevel", "8", "-threads", "auto"}
	subtitle_number := *subtitle_int

	// Create grayscale FFmpeg - options
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

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Test that all input files have a video stream and that the audio and subtitle streams the user wants do exist //
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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

		// If user gave audio stream number, check that we have at least that much audio streams in the source file.
		if len(audio_slice) - 1 < *audio_stream_number_int {
			error_messages = append(error_messages, "File: '" + file_name + "' does not have an audio stream number: " + strconv.Itoa(*audio_stream_number_int))
		}

		// If user gave subtitle stream number, check that we have at least that much subtitle streams in the source file.
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
				audio_codec = audio_info[4]

				fmt.Printf("Audio stream number: %d, language: %s, for visually impared: %s, number of channels: %s, audio codec: %s\n", audio_stream_number, audio_language, for_visually_impared, number_of_audio_channels, audio_codec)
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

	////////////////////////////////////////////////////////////////////////////////////////////////////
	// If user gave us the audio language (fin, eng, ita), find the corresponding audio stream number //
	// If no matching audio is found stop the program.                                                //
	////////////////////////////////////////////////////////////////////////////////////////////////////
	if *audio_language_str != "" {

		for _,file_info_slice := range Complete_file_info_slice {

			video_slice_temp := file_info_slice[0]
			video_slice := video_slice_temp[0]
			file_name := video_slice[0]
			audio_slice := file_info_slice[1]
			audio_stream_found := false

			for _, audio_info := range audio_slice {
				audio_language = audio_info[0]

				if *audio_language_str == audio_language {
					audio_stream_found = true
					break // Continue searching the next file when the first matching audio language has been found.
				}

			}
			if audio_stream_found == false {
				fmt.Println()
				fmt.Printf("Error, could not find audio language: %s in file: %s\n", *audio_language_str, file_name)
				fmt.Println("Scan the file for possible audio languages with the -scan option.")
				fmt.Println()
				os.Exit(0)
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Audio: %s was found in file %s\n", *audio_language_str, file_name)
				fmt.Println()
			}
		}

	}

	//////////////////////////////////////////////////////////////////////////////////////////////////////////
	// If user gave us the subtitle language (fin, eng, ita), find the corresponding subtitle stream number //
	// If no matching subtitle is found stop the program.                                                   //
	//////////////////////////////////////////////////////////////////////////////////////////////////////////
	if *subtitle_language_str != "" {

		for _,file_info_slice := range Complete_file_info_slice {

			video_slice_temp := file_info_slice[0]
			video_slice := video_slice_temp[0]
			file_name := video_slice[0]
			subtitle_slice := file_info_slice[2]
			subtitle_found := false

			for _, subtitle_info := range subtitle_slice {
				subtitle_language = subtitle_info[0]

				if *subtitle_language_str == subtitle_language {
					subtitle_found = true
					break // Continue searching the next file when the first matching subtitle has been found.
				}

			}

			if subtitle_found == false {
				fmt.Println()
				fmt.Printf("Error, could not find subtitle language: '%s' in file: %s\n", *subtitle_language_str, file_name)
				fmt.Println("Scan the file for possible subtitle languages with the -scan option.")
				fmt.Println()
				os.Exit(0)
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Subtitle: %s was found in file %s\n", *subtitle_language_str, file_name)
				fmt.Println()
			}
		}

	}

	/////////////////////////////////////////
	// Main loop that processess all files //
	/////////////////////////////////////////

	files_to_process_str = strconv.Itoa(len(Complete_file_info_slice))

	for _,file_info_slice := range Complete_file_info_slice {

		subtitle_horizontal_offset_int = 0
		subtitle_horizontal_offset_str = "0"
		start_time = time.Now()
		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]
		file_name := video_slice[0]
		video_width = video_slice[1]
		video_height = video_slice[2]
		video_duration = video_slice[3]

		// Create input + output filenames and paths
		inputfile_absolute_path,_ := filepath.Abs(file_name)
		inputfile_path := filepath.Dir(inputfile_absolute_path)
		inputfile_name := filepath.Base(file_name)
		input_filename_extension := filepath.Ext(inputfile_name)
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension) + output_filename_extension)

		if *debug_mode_on == true {
			fmt.Println("inputfile_path:", inputfile_path)
			fmt.Println("inputfile_name:", inputfile_name)
			fmt.Println("output_file_absolute_path:", output_file_absolute_path)
			fmt.Println("video_width:", video_width)
			fmt.Println("video_height:", video_height)
		}

		// Add messages to processing log.
		var log_messages_str_slice []string
		log_messages_str_slice = append(log_messages_str_slice, "")
		log_messages_str_slice = append(log_messages_str_slice, "Filename: " + file_name)
		underline_length := len(file_name) + len ("Filename: ") + 1
		log_messages_str_slice = append(log_messages_str_slice, strings.Repeat("-", underline_length))
		log_messages_str_slice = append(log_messages_str_slice, "")
		log_messages_str_slice = append(log_messages_str_slice, "Commandline options:")
		log_messages_str_slice = append(log_messages_str_slice, "---------------------")
		log_messages_str_slice = append(log_messages_str_slice, strings.Join(os.Args, " "))

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

		//////////////////////////////////////////////////////////////////////////////////////////////////////
		// Find audio number corresponding to the audio language name (eng, fin, ita) user possibly gave us //
		//////////////////////////////////////////////////////////////////////////////////////////////////////
		if *audio_language_str != "" {

			audio_slice := file_info_slice[1]
			audio_stream_found := false

			for audio_stream_number, audio_info := range audio_slice {
				audio_language = audio_info[0]

				if *audio_language_str == audio_language {
					*audio_stream_number_int = audio_stream_number
					number_of_audio_channels = audio_info[2]
					audio_stream_found = true
					break // Break out of the loop when the first matching audio has been found.
				}

			}

			if audio_stream_found == false {
				fmt.Println()
				fmt.Printf("Error, could not find audio language: %s in file: %s\n", *audio_language_str, file_name)
				fmt.Println("Scan the file for possible audio languages with the -scan option.")
				fmt.Println()
				os.Exit(0)
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Audio: %s was found in audio stream number: %d\n", *audio_language_str, *audio_stream_number_int)
				fmt.Println()
			}

		} else {
			audio_slice := file_info_slice[1]
			audio_info := audio_slice[*audio_stream_number_int]
			number_of_audio_channels = audio_info[2]
		}

		// Test if output audio codec is compatible with the mp4 wrapper format
		audio_slice := file_info_slice[1]
		audio_info := audio_slice[*audio_stream_number_int]
		audio_codec = audio_info[4]

		if *audio_compression_ac3 == true {
			audio_codec = "ac3"
		}

		if *use_matroska_container == false {

			if audio_codec != "aac" && audio_codec != "ac3" && audio_codec != "mp2" && audio_codec != "mp3" &&audio_codec != "dts" {
				fmt.Println()
				fmt.Printf("Error, audio codec: '%s' in file: %s is not compatible with the mp4 wrapper format.\n", audio_codec, file_name)
				fmt.Println("The compatible audio formats are: aac, ac3, mp2, mp3, dts.")
				fmt.Println("")
				fmt.Println("You have three options:")
				fmt.Println("1. Use the -scan option to find which input files have incompatible audio and process these files separately.")
				fmt.Println("2. Use the -ac3 option to compress audio to ac3.")
				fmt.Println("3. Use the -mkv option to use matroska as the output file wrapper format.")
				fmt.Println()
				os.Exit(0)
			}
		}

		////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Find subtitle number corresponding to the subtitle language name (eng, fin, ita) user possibly gave us //
		////////////////////////////////////////////////////////////////////////////////////////////////////////////
		if *subtitle_language_str != "" {

			subtitle_slice := file_info_slice[2]
			subtitle_found := false

			for subtitle_stream_number, subtitle_info := range subtitle_slice {
				subtitle_language = subtitle_info[0]

				if *subtitle_language_str == subtitle_language {
					subtitle_number = subtitle_stream_number
					subtitle_found = true
					break // Stop searching when the first matching subtitle has been found.
				}

			}

			if subtitle_found == false {
				fmt.Println()
				fmt.Printf("Error, could not find subtitle language: '%s' in file: %s\n", *subtitle_language_str, file_name)
				fmt.Println("Scan the file for possible subtitle languages with the -scan option.")
				fmt.Println()
				os.Exit(0)
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Subtitle: %s was found in subtitle stream number: %d\n", *subtitle_language_str, subtitle_number)
				fmt.Println()
			}
		}

		/////////////////////////////////////////////////////////////
		// Find out autocrop parameters by scanning the input file //
		/////////////////////////////////////////////////////////////

		// FFmpeg cropdetect scans the file and tries to guess where the black bars are.
		// The command: cropdetect=24:8:250  means:
		//
		// Threshold for black is 24.
		// The values returned by cropdetect must be divisible by 8.
		// FFmpeg recommends using video sizes divisible by 16 for most video codecs.
		// We use 8 here since in 1920x1080 the 1080 is not divisible by 16 still 1920x1080 is a stardard H.264 resolution, so the codec handles that resolution ok.
		// Reset detected border values to zero after 250 frames and try to detect borders again.
		//
		// FFmpeg returns a bunch of measurements like this: crop=1472:1080:224:0
		// Lets see what this means and replace the values by variables: crop=A:B:C:D
		// The line tells us what part of the picture will be left over after cropping. The line means:
		//
		// The detected left border is at C pixels from the left of the picture.
		// Take A pixels starting from C to the right and where we end at is the detected right border of the picture.
		// Pixels on the right of this point will be cropped.
		//
		// The detected upper border is at D pixels from the top of the picture
		// Take B pixels starting from D down and where we end at is the detected bottom border of the picture.
		// Pixels below this point will be cropped.
		// 

		if *autocrop_bool == true {

			// Create the FFmpeg commandline to scan for black areas at the borders of the video.
			command_to_run_str_slice = nil
			quick_scan_failed := false

			// Clear crop value storage map by creating a new map with the same name.
			var crop_value_map = make(map[string]int)

			video_duration_int,_ := strconv.Atoi(strings.Split(video_duration, ".")[0])

			// For long videos take short snapshots of crop values spanning the whole file. This is "quick scan mode".
			if video_duration_int > 300 {

				spotcheck_interval := video_duration_int / 10 // How many spot checks will be made across the duration of the video (default = 10)
				scan_duration_str := "10" // How many seconds of video to scan for each spot (default = 10 seconds)
				scan_duration_int,_ := strconv.Atoi(scan_duration_str)

				if *debug_mode_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				// Repeat spot checks
				for time_to_jump_to := scan_duration_int ; time_to_jump_to + scan_duration_int < video_duration_int ; time_to_jump_to = time_to_jump_to + spotcheck_interval {

					// Create the ffmpeg command to scan for crop values
					command_to_run_str_slice = nil
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-ss", strconv.Itoa(time_to_jump_to), "-t", scan_duration_str, "-i", file_name, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

					if *debug_mode_on == true {
						fmt.Println()
						fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
						fmt.Println()
					}

					ffmpeg_crop_output, ffmpeg_crop_error := run_external_command(command_to_run_str_slice)

					// Parse the crop value list to find the value that is most frequent, that is the value that can be applied without cropping too much or too little.
					if ffmpeg_crop_error == nil {

						crop_value_counter := 0

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
					} else {
						fmt.Println()
						fmt.Println("Quick scan for crop failed, switching to the slow method")
						fmt.Println()
						quick_scan_failed = true
						break
					}
				}
			}

			// Scan the file for crop values.
			if video_duration_int < 300 || quick_scan_failed == true || len(crop_value_map) == 0 {

				command_to_run_str_slice = nil
				command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-t", "1800", "-i", file_name, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

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

					crop_value_counter := 0

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
				} else {
					fmt.Println()
					fmt.Println("Scanning inputfile with FFmpeg resulted in an error:")
					fmt.Println(ffmpeg_crop_error)
					os.Exit(1)
				}
			}

			// Find the most frequent crop value
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

			video_height_int, _  := strconv.Atoi(video_height)
			cropped_height := video_height_int - crop_values_picture_height - crop_values_height_offset

			video_width_int, _  := strconv.Atoi(video_width)
			cropped_width := video_width_int - crop_values_picture_width - crop_values_width_offset

			// Prepare offset for possible subtitle burn in
			// Subtitle placement is always relative to the left side of the picture,
			// if left is cropped then the subtitle needs to be moved left the same amount of pixels
			subtitle_horizontal_offset_int = crop_values_width_offset * -1
			subtitle_horizontal_offset_str = strconv.Itoa(subtitle_horizontal_offset_int)

			fmt.Println("Top:", crop_values_height_offset, ", Bottom:", strconv.Itoa(cropped_height), ", Left:", crop_values_width_offset, ", Right:", strconv.Itoa(cropped_width))

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "Crop values are, Top: " + strconv.Itoa(crop_values_height_offset) + ", Bottom: " + strconv.Itoa(cropped_height) + ", Left: " + strconv.Itoa(crop_values_width_offset) + ", Right: " + strconv.Itoa(cropped_width))
			log_messages_str_slice = append(log_messages_str_slice, "After cropping video width is: " + strconv.Itoa(crop_values_picture_width) + ", and height is: " + strconv.Itoa(crop_values_picture_height))

		}

		/////////////////////////
		// Encode video - mode //
		/////////////////////////

		if *scan_mode_only_bool == false {

			ffmpeg_pass_1_commandline = nil
			ffmpeg_pass_2_commandline = nil

			// Create the start of ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_commandline_start...)

			// If the user wants to use the fast and inaccurate search, place the -ss option before the first -i on ffmpeg commandline.
			if *search_start_str != "" && *fast_search_bool == true {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", *search_start_str)
			}

			// Add possible dvd subtitle color palette hacking option to the FFmpeg commandline.
			// It must be before the first input file to take effect for that file.
			if *subtitle_palette != "" && *subtitle_mux_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-palette", *subtitle_palette)
			}

			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", file_name)

			// The user wants to use the slow and accurate search, place the -ss option after the first -i on ffmpeg commandline.
			if *search_start_str != "" && *fast_search_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", *search_start_str)
			}

			if *processing_time_str != "" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-t", *processing_time_str)
			}

			ffmpeg_filter_options := ""

			/////////////////////////////////////////////////////////////////////////////////////////////////////////
			// If there is no subtitle to process or we are just muxing dvd, dvb or bluray subtitle to target file //
			// then use the simple video processing chain (-vf) in FFmpeg                                          //
			// It has a processing pipeline with only one video input and output                                   //
			/////////////////////////////////////////////////////////////////////////////////////////////////////////

			if subtitle_number == -1 || *subtitle_mux_bool == true {

				if *subtitle_mux_bool == true {
					// There is a dvd, dvb or bluray bitmap subtitle to mux into the target file add the relevant options to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-scodec", "copy", "-map", "0:s:" + strconv.Itoa(subtitle_number))
				} else {
					// There is no subtitle to process add the "no subtitle" option to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-sn")
				}

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
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
				// There is a subtitle to burn into the video, use the complex video processing chain in FFmpeg (-filer_complex) //
				// It can have several simultaneous video inputs and outputs.                                                    //
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
				"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
				strconv.Itoa(*subtitle_vertical_offset_int) + strings.Join(grayscale_options, "") + "[processed_combined_streams]", "-map", "[processed_combined_streams]")
			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compressing options to FFmpeg commandline //
			///////////////////////////////////////////////////////////////////

			// If video vertical resolution is over 700 pixel choose HD video compression settings
			video_compression_options := video_compression_options_sd

			video_height_int,_ = strconv.Atoi(video_height)

			// If video has been cropped, decide video compression bitrate  by the cropped hight of the video.
			if *autocrop_bool == true {
				video_height_int = crop_values_picture_height
			}

			video_bitrate = "1600k"

			if *force_hd_bool == true || video_height_int > 700 {
				video_compression_options = video_compression_options_hd
				video_bitrate = "8000k"
			}

			if *force_lossless_bool == true {
				// Lossless audio compression options
				audio_compression_options = nil
				audio_compression_options = audio_compression_options_lossless_flac

				// Lossless video compression options
				video_compression_options = video_compression_options_lossless
				video_bitrate = "Lossless"
			}

			if *audio_compression_ac3 == true {

				number_of_audio_channels_int,_ := strconv.Atoi(number_of_audio_channels)

				if number_of_audio_channels_int <= 2 {
					audio_compression_options = nil
					audio_compression_options = audio_compression_options_2_channels_ac3
				} else {
					audio_compression_options = nil
					audio_compression_options = audio_compression_options_6_channels_ac3
				}
			}

			// Add video compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, video_compression_options...)

			// Add audio compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, audio_compression_options...)
			
			// Add audiomapping options on the commanline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(*audio_stream_number_int))

			// Add 2 - pass logfile path to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-passlogfile")
			ffmpeg_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_2_pass_logfile_path)
		
			// Add video output format to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_video_format...)

			// Copy ffmpeg pass 2 commandline to ffmpeg pass 1 commandline
			ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, ffmpeg_pass_2_commandline...)

			// Add pass 1/2 info on ffmpeg commandline
			if *fast_encode_bool == false {

				ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "-pass", "1")
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-pass", "2")

				// Add /dev/null output option to ffmpeg pass 1 commandline
				ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "/dev/null")
			}

			// Add outfile path to ffmpeg pass 2 commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_file_absolute_path)

			// If we have "fast" mode on then we will only do 1-pass encoding and the pass 1 commanline is the same as pass 2.
			// In this case we won't do pass 2 at all.
			if *fast_encode_bool == true {
				ffmpeg_pass_1_commandline = ffmpeg_pass_2_commandline
			}

			/////////////////////////////////////////
			// Print variable values in debug mode //
			/////////////////////////////////////////
			if *debug_mode_on == true {
				fmt.Println()
				fmt.Println("video_compression_options_sd:",video_compression_options_sd)
				fmt.Println("video_compression_options_hd:",video_compression_options_hd)
				fmt.Println("video_compression_options:",video_compression_options)
				fmt.Println("audio_compression_options:", audio_compression_options)
				fmt.Println("denoise_options:",denoise_options)
				fmt.Println("deinterlace_options:",deinterlace_options)
				fmt.Println("ffmpeg_commandline_start:",ffmpeg_commandline_start)
				fmt.Println("subtitle_number:",subtitle_number)
				fmt.Println("subtitle_language_str:",subtitle_language_str)
				fmt.Println("subtitle_vertical_offset_int:",*subtitle_vertical_offset_int)
				fmt.Println("*subtitle_downscale:",*subtitle_downscale)
				fmt.Println("*subtitle_palette:",*subtitle_palette)
				fmt.Println("*subtitle_mux_bool:",*subtitle_mux_bool)
				fmt.Println("*grayscale_bool:", *grayscale_bool)
				fmt.Println("grayscale_options:",grayscale_options)
				fmt.Println("*autocrop_bool:", *autocrop_bool)
				fmt.Println("*subtitle_int:", *subtitle_int)
				fmt.Println("*no_deinterlace_bool:", *no_deinterlace_bool)
				fmt.Println("*denoise_bool:", *denoise_bool)
				fmt.Println("*force_hd_bool:", *force_hd_bool)
				fmt.Println("*audio_stream_number_int:", *audio_stream_number_int)
				fmt.Println("*scan_mode_only_bool", *scan_mode_only_bool)
				fmt.Println("*search_start_str", *search_start_str)
				fmt.Println("*processing_time_str", *processing_time_str)
				fmt.Println("*fast_bool", *fast_bool)
				fmt.Println("*fast_search_bool", *fast_search_bool)
				fmt.Println("*fast_encode_bool", *fast_encode_bool)
				fmt.Println("*debug_mode_on", *debug_mode_on)
				fmt.Println()
				fmt.Println("input_filenames:", input_filenames)
			}

			/////////////////////////////////////
			// Run Pass 1 encoding with FFmpeg //
			/////////////////////////////////////
			if *debug_mode_on == true {

				fmt.Println()
				fmt.Println("ffmpeg_pass_1_commandline:", ffmpeg_pass_1_commandline)

			} else {
				fmt.Println()
				fmt.Println("Encoding with video bitrate:", video_bitrate)

				if *audio_compression_ac3 == true {

					fmt.Println("Encoding audio to ac3 with bitrate:", audio_compression_options[3])

				} else {

					fmt.Printf("Copying %s audio to target.\n", audio_codec)
				}

				fmt.Printf("Pass 1 encoding: " + inputfile_name + " ")
			}

			pass_1_start_time = time.Now()

			ffmpeg_pass_1_output_temp, ffmpeg_pass_1_error := run_external_command(ffmpeg_pass_1_commandline)

			pass_1_elapsed_time = time.Since(pass_1_start_time)
			fmt.Printf("took %s", pass_1_elapsed_time.Round(time.Millisecond))
			fmt.Println()

			// Add messages to processing log.
			pass_1_commandline_for_logfile := strings.Join(ffmpeg_pass_1_commandline, " ")

			// Make a copy of the FFmpeg commandline for writing in the logfile.
			// Modify commandline so that it works if the user wants to copy and paste it from the logfile and run it.
			if subtitle_number == -1 || *subtitle_mux_bool == true {
				// Simple processing chain with -vf.
				pass_1_commandline_for_logfile = strings.Replace(pass_1_commandline_for_logfile, "-vf ", "-vf '", 1)
				pass_1_commandline_for_logfile = strings.Replace(pass_1_commandline_for_logfile, " -c:v", "' -c:v", 1)
			} else {
				// Complex processing chain with -filter_complex
				pass_1_commandline_for_logfile = strings.Replace(pass_1_commandline_for_logfile, "-filter_complex ", "-filter_complex '", 1)
				pass_1_commandline_for_logfile = strings.Replace(pass_1_commandline_for_logfile, "[processed_combined_streams] -map", "[processed_combined_streams]' -map", 1)
			}

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 1 Options:")
			log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
			log_messages_str_slice = append(log_messages_str_slice, pass_1_commandline_for_logfile)

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

			/////////////////////////////////////
			// Run Pass 2 encoding with FFmpeg //
			/////////////////////////////////////
			if *fast_encode_bool == false {

				if *debug_mode_on == true {

					fmt.Println()
					fmt.Println("ffmpeg_pass_2_commandline:", ffmpeg_pass_2_commandline)

				} else {

					pass_2_elapsed_time = time.Since(start_time)
					fmt.Printf("Pass 2 encoding: " + inputfile_name + " ")
				}

				pass_2_start_time = time.Now()

				ffmpeg_pass_2_output_temp, ffmpeg_pass_2_error :=  run_external_command(ffmpeg_pass_2_commandline)

				pass_2_elapsed_time = time.Since(pass_2_start_time)
				fmt.Printf("took %s", pass_2_elapsed_time.Round(time.Millisecond))
				fmt.Println()

				// Add messages to processing log.
				pass_2_commandline_for_logfile := strings.Join(ffmpeg_pass_2_commandline, " ")
				pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, "[0:s:0]", "'[0:s:0]", 1)
				pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, "[processed_combined_streams] -map", "[processed_combined_streams]' -map", 1)
				log_messages_str_slice = append(log_messages_str_slice, "")
				log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 2 Options:")
				log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
				log_messages_str_slice = append(log_messages_str_slice, pass_2_commandline_for_logfile)

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

			// Add messages to processing log.
			log_messages_str_slice = append(log_messages_str_slice, "")
			pass_1_elapsed_time := pass_1_elapsed_time.Round(time.Millisecond)
			pass_2_elapsed_time := pass_2_elapsed_time.Round(time.Millisecond)
			total_elapsed_time := elapsed_time.Round(time.Millisecond)
			log_messages_str_slice = append(log_messages_str_slice, "Pass 1 took: " + pass_1_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "Pass 2 took: " + pass_2_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "Processing took: " + total_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "########################################################################################################################")
			log_messages_str_slice = append(log_messages_str_slice, "")
		}

		// Open logfile for appending info about file processing to it.
		log_file_name := "00-processing.log"
		log_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, log_file_name)

		// Append to the logfile or if it does not exist create a new one.
		logfile_pointer, err := os.OpenFile(log_file_absolute_path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
		defer logfile_pointer.Close()

		if err != nil {
			fmt.Println("")
			fmt.Println("Error, could not open logfile:", log_file_name, "for writing.")
			log.Fatal(err)
			os.Exit(0)
		}

		// Write processing info to the file
		if _, err = logfile_pointer.WriteString(strings.Join(log_messages_str_slice, "\n")); err != nil {
			fmt.Println("")
			fmt.Println("Error, could not write to logfile:", log_file_name)
			log.Fatal(err)
			os.Exit(0)
		}
	}
}

// FIXME
// Jos ohjelmalle annetusta tiedostojoukosta puuttuu yksi failia, ohjelma exitoi eikä käsittele yhtään tiedostoa.
// Jos kroppausarvot on nolla, poista kroppaysoptiot ffmpegin komentoriviltä ?
// Tee enkoodauksen aikainen FFmpegin tulosteen tsekkaus, joka laskee koodauksen aika-arvion ja prosentit siitä kuinka paljon failia on jo käsitelty (fps ?) Tästä on esimerkkiohjelma muistiinpanoissa, mutta se jumittaa n. 90 sekuntia FFmpeg - enkoodauksen alkamisesta.
// Nimeä ffmpeg_enkooderi uudella nimellä (sl_encoder = starlight encoder) ja poista hakemisto: 00-vanhat jotta git repon voi julkaista
// Tee erillinen skripti audion synkkaamista varten
// Tsekkaa pitäiskö aina --filter_complexin kanssa käyttää audiossa oletus-delaytä (ehkä 40 ms).
// Pitäiskö laittaa option, jolla vois rajoittaa käytettävien prosessorien lukumäärän ?
//
// Mites dts-hd ?:
//
// Tee -an optio, joka pakkaa ainoastaan videon ja jättää audio kokonaan pois.
// Pitäiskö tehdä mahdollisuus muxata kohdetiedostoon useamman kielinen tekstitys ? Tässä tulis kohtuullisen isoja koodimuutoksia.
//
// Copy 1 minute starting from 10 minutes of a mkv to a new container: ffmpeg -i InputFile.mkv -ss 10:00 -t 01:00 -vcodec copy -acodec copy -scodec copy -map 0 OutputFile.mkv
//
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+140[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 1 /dev/null
//
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+70[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4
//
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=(main_w-overlay_w)/2:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4
//
//
// Muuta nykyinen subtitle scale optio (-sd), joksin muuksi, esim. -scr (subtitle crop resize) tai -sca (subtitle crop adjust). Sitten tee uudet optiot: -ssd (subtitle scale down) -ssu (subtitle scale up), -shc (subtitle horizontal center). muuta nykyinen optio -so optioksi -svo (subtitle vertical offset) ja tee uusi optio: -sho (subtitle horizontal offset).
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 01:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=((main_w-overlay_w)/2)+30:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 1 /dev/null
//
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 01:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=((main_w-overlay_w)/2)+30:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4

// Processing file 1/2  'Tales_Of_Tomorrow.mkv'
// Finding crop values for: Tales_Of_Tomorrow.mkv   Top: 6 , Bottom: 2 , Left: 6 , Right: 2
// 
// Encoding with video bitrate: 1600k
// Pass 1 encoding: Tales_Of_Tomorrow.mkv took 5m11.383s
// Pass 2 encoding: Tales_Of_Tomorrow.mkv took 6m55.197s
// Processing took 12m14.406s
// 
// ################################################################################
// 
// Processing file 2/2  'palkintojen_jakojuhla.mkv'
// 
// Error, audio codec: 'pcm_s16le' in file: palkintojen_jakojuhla.mkv is not compatible with the mp4 wrapper format.
// The compatible audio formats are: aac, ac3, mp2, mp3, dts.
// 
// You have three options:
// 1. Use the -scan option to find which input files have incompatible audio and process these files separately.
// 2. Use the -ac3 option to compress audio to ac3.
// 3. Use the -mkv option to use matroska as the output file wrapper format.


