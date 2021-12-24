// (C) Mikael Hartzell 2018
// This program is distributed under the GNU General Public License, version 3 (GPLv3)

package main

import (
	"bytes"
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)


// Global variable definitions

//////////////////////////////////////////////////////////////////////////////////////////
// Defaults. Edit these to change program behavior                                      //
//////////////////////////////////////////////////////////////////////////////////////////
// Define default x264 processing profiles for different resolutions
// Profiles are automatically selected based on video resolution
var video_compression_options_sd = []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "main", "-level", "4.0"}
var video_compression_options_hd = []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "4.1"}
var video_compression_options_ultra_hd_4k = []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "6.1"}
var video_compression_options_ultra_hd_8k = []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "6.2"}
var video_compression_options_lossless = []string{"-c:v", "utvideo"}
var audio_compression_options = []string{"-acodec", "copy"}
var audio_compression_options_lossless = []string{"-acodec", "flac"}
var denoise_options = []string{"hqdn3d=3.0:3.0:2.0:3.0"}
var color_subsampling_options = []string{"-pix_fmt", "yuv420p"}

// Video compression bitrate is calculated like this:
// (Horixontal resolution * vertical resolution) / video_compression_bitrate_divider.
// For example: 1920 x 1080 = 2 073 600 pixels / 256 = bitrate 8100k
var video_compression_bitrate_divider = 256

// Constant Quality CRF uses as much bitrate as is needed
// to have the video quality be constant throughout the video.
// CRF compression is much faster than 2-pass but it creates a larger file.
// Also some dark scenes might look better on 2-pass than CRF.
// 17 or 18 is recommended for FFmpeg to create a copy with almost the same quality as the original
// although there is always some detail loss on recompression on any bitrate.
// Smaller value creates a bigger file
// 17 and 18 look the same to me and both seem equal to 2-pass with the default compression bitrates used in this program
var crf_value = "18"

// Default video processing. Possible values are "2-pass" and "crf"
var default_video_processing = "2-pass" 
// var default_video_processing = "crf" 

// Default audio processing. Possible values are "copy", "opus" and "aac"
// Copy copies the original audio unchanhed from source file to target,
// opus recompresses audio to opus - format
// and aac recompresses audio to aac - format
var default_audio_processing = "copy"
// var default_audio_processing = "opus"
// var default_audio_processing = "aac"

// Default audio bitrate per channel is 128k. If there are 6 channels then this results to 128 * 6 = 768k
var audio_bitrate_multiplier = 128

// Default number of thread to use. There are claims on the internet that using more than 8 threads
// in h264 processing will hurt quality, because the threads can not use results from other
// threads to optimize quality. This is why we default to using a maximum of 8 threads,
// except when creating a main (HD) and SD video simultaneously
// When using imagemagick to resize subtitles all cores are always used.
//
// Possible values are:
// "" empty parenthesis means calculate thread count automatically based on how many cores the computer has, and use a max of 8.
// a number like "12" means always use twelve threads,
// the word "auto" means let FFmpeg decide how many threads to use.
var default_max_threads = ""
//var default_max_threads = "12"
//var default_max_threads = "auto"

//////////////////////////////////////////////////////////////////////////////////////////
// Defaults ends here                                                                   //
//////////////////////////////////////////////////////////////////////////////////////////



var version_number string = "2.45" // This is the version of this program
var Complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)
var helptext_categories_map = make(map[string][]string) // The key is commandline option category (video, audio, subtitle) and the slice contains commandline options that belong to this category
var commandline_option_map = make(map[string]*commandline_struct) // The key is the commandline option and the struct contains all variables and helptext belonging to that option
var debug_option *bool

type commandline_struct struct {
	is_turned_on bool
	option_type string
	user_int int
	user_string string
	help_text string
}

// Create a slice for storing all video, audio and subtitle stream infos for each input file.
// There can be many audio and subtitle streams in a file.
var Complete_file_info_slice [][][][]string

func run_external_command(command_to_run_str_slice []string) (stdout_output []string, stderr_output []string, error_code error) {

	command_output_str := ""
	stderror_output_str := ""

	// Create the struct needed for running the external command
	command_struct := exec.Command(command_to_run_str_slice[0], command_to_run_str_slice[1:]...)

	// Run external command
	var stdout, stderr bytes.Buffer
	command_struct.Stdout = &stdout
	command_struct.Stderr = &stderr

	error_code = command_struct.Run()

	command_output_str = string(stdout.Bytes())
	stderror_output_str = string(stderr.Bytes())

	// Split the output of the command to lines and store in a slice
	for _, line := range strings.Split(command_output_str, "\n") {
		stdout_output = append(stdout_output, line)
	}

	// Split the output of the stderr to lines and store in a slice
	for _, line := range strings.Split(stderror_output_str, "\n") {
		stderr_output = append(stderr_output, line)
	}

	return stdout_output, stderr_output, error_code
}

func find_executable_path(filename string) (file_path string) {

	/////////////////////////////////////////////////
	// Test if executable can be found in the path //
	/////////////////////////////////////////////////

	if _, error := exec.LookPath(filename); error != nil {
		fmt.Println()
		fmt.Println("Error, cant find program: " + filename + " in path, can't continue.")


		if filename == "magick" || filename == "mogrify" {
			fmt.Println(filename, "is part of package ImageMagick and is needed for the -sp functionality.")
		}

		fmt.Println()
		os.Exit(1)
	}

	return file_path
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

	for _, text_line := range unsorted_ffprobe_information_str_slice {
		stream_number_int = -1
		stream_info_str_slice = nil

		// If there are many programs in the file, then the stream information is listed twice by ffprobe,
		// discard duplicate data.
		if strings.HasPrefix(text_line, "programs.program") {
			continue
		}

		if strings.HasPrefix(text_line, "streams.stream") {

			text_line_str_slice = strings.Split(strings.Replace(text_line, "streams.stream.", "", 1), ".")

			// Convert stream number from string to int
			error = nil

			if stream_number_int, error = strconv.Atoi(text_line_str_slice[0]); error != nil {
				// Stream number could not be understood, skip the stream
				continue
			}

			// Remove the text "streams.stream." from the beginning of each text line
			string_to_remove_str_slice = string_to_remove_str_slice[:0] // Clear the slice so that allocated slice ram space remains and is not garbage collected.
			string_to_remove_str_slice = append(string_to_remove_str_slice, "streams.stream.", strconv.Itoa(stream_number_int), ".")
			stream_data_str = strings.Replace(text_line, strings.Join(string_to_remove_str_slice, ""), "", 1) // Remove the unwanted string in front of the text line.
			stream_data_str = strings.Replace(stream_data_str, "\"", "", -1)                                  // Remove " characters from the data.

			// Add found stream info line to a slice with previously stored info
			// and store it in a map. The stream number acts as the map key.
			if _, item_found := Complete_stream_info_map[stream_number_int]; item_found == true {
				stream_info_str_slice = Complete_stream_info_map[stream_number_int]
			}
			stream_info_str_slice = append(stream_info_str_slice, stream_data_str)
			Complete_stream_info_map[stream_number_int] = stream_info_str_slice
		}

		// Get media file wrapper information and store it in a slice.
		if strings.HasPrefix(text_line, "format") {
			wrapper_info_str_slice = strings.Split(strings.Replace(text_line, "format.", "", 1), "=")
			wrapper_key := strings.TrimSpace(wrapper_info_str_slice[0])
			wrapper_value := strings.TrimSpace(strings.Replace(wrapper_info_str_slice[1], "\"", "", -1)) // Remove whitespace and " charcters from the data
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

	for key := range Complete_stream_info_map {
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

			devidend_str := ""
			devisor_str := ""
			devidend_int := 0
			devisor_int := 0
			var error_happened error
			var result float64

			frame_rate_str := video_stream_info_map["r_frame_rate"]
			frame_rate_average_str := video_stream_info_map["avg_frame_rate"]

			// Frame rate may be displayed by ffprobe in the form of a division like: 300000/1001. Do the calculation to get the human redable frame rate.
			index := strings.Index(frame_rate_str, "/")

			if index > 0 {
				temp_slice := strings.Split(frame_rate_str, "/")
				devidend_str = temp_slice[0]
				devisor_str = temp_slice[1]

				devidend_int, error_happened = strconv.Atoi(devidend_str)

				if error_happened == nil {
					devisor_int, error_happened = strconv.Atoi(devisor_str)
				}

				if error_happened == nil {
					result = float64(devidend_int) / float64(devisor_int)
					frame_rate_str = strconv.FormatFloat(result, 'f', 3, 64)
				}
			}

			if error_happened != nil {
				fmt.Println("Info: could not convert frame rate info from ffprobe's output to integer")
			}

			// Average frame rate may be displayed by ffprobe in the form of a division like: 300000/1001. Do the calculation to get the human redable frame rate.
			devidend_str = ""
			devisor_str = ""
			devidend_int = 0
			devisor_int = 0
			result = 0.0
			error_happened = nil

			index = strings.Index(frame_rate_average_str, "/")

			if index > 0 {
				temp_slice := strings.Split(frame_rate_average_str, "/")
				devidend_str = temp_slice[0]
				devisor_str = temp_slice[1]

				devidend_int, error_happened = strconv.Atoi(devidend_str)

				if error_happened == nil {
					devisor_int, error_happened = strconv.Atoi(devisor_str)
				}

				if error_happened == nil {
					result = float64(devidend_int) / float64(devisor_int)
					frame_rate_average_str = strconv.FormatFloat(result, 'f', 3, 64)
				}
			}

			if error_happened != nil {
				fmt.Println("Info: could not convert average frame rate info from ffprobe's output to integer")
			}

			// Add also duration from wrapper information to the video info.
			single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, video_stream_info_map["width"], video_stream_info_map["height"], wrapper_info_map["duration"], video_stream_info_map["codec_name"], video_stream_info_map["pix_fmt"], video_stream_info_map["color_space"], frame_rate_str, frame_rate_average_str)
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
	// The contents is when info for one file is stored: [ [ [/home/mika/Downloads/dvb_stream.ts 720 576 64.123411, h264, yuv420p, bt709, 25, 24]]  [[eng 0 2 48000 ac3]  [dut 1 2 48000 pcm_s16le]]  [[fin 0 dvb_subtitle]  [fin 0 dvb_teletext] ] ]
	//
	// The file path is: /home/mika/Downloads/dvb_stream.ts
	// Video width is: 720 pixels and height is: 576 pixels and the duration is: 64.123411 seconds.
	// Video codec is: h264
	// Color subsampling is: yuv420p
	// Color space: bt709
	// Frame rate is 25
	// Average frame rate is 24
	//
	// The input file has two audio streams (languages: eng and dut)
	// Audio stream 0: language is: english, audio is for for visually impared = 0 (false), there are 2 audio channels in the stream and sample rate is 48000 and audio codec is ac3.
	// Audio stream 1: language is: dutch, audio is for visually impared = 1 (true), there are 2 audio channels in the stream and sample rate is 48000 and audio codec is pcm_s16le.
	//
	// The input file has two subtitle streams
	// Subtitle stream 0: language is: finnish, subtitle is for hearing impared = 0 (false), the subtitle codec is: dvb (bitmap)
	// Subtitle stream 1: language is: finnish, subtitle is for hearing impared = 0 (false), the subtitle codec is: teletext
	//

	return
}

func convert_timecode_to_seconds(timestring string) (string, string) {
	var hours_int, minutes_int, seconds_int, seconds_total_int int
	var hours_str, minutes_str, seconds_str, milliseconds_str string
	var seconds_total_str, error_happened string

	if strings.ContainsAny(timestring, ".") {
		milliseconds_str = strings.Split(timestring, ".")[1]
		timestring = strings.Replace(timestring, "." + milliseconds_str, "", 1)

		// Truncate milliseconds to 3 digits
		if len(milliseconds_str) > 3 {
			milliseconds_str = milliseconds_str[0:3]
		}
	}

	temp_str_slice := strings.Split(timestring, ":")

	if len(temp_str_slice) == 3 {
		hours_str = temp_str_slice[0]
		hours_int, _ = strconv.Atoi(hours_str)
		minutes_str = temp_str_slice[1]
		minutes_int, _ = strconv.Atoi(minutes_str)
		seconds_str = temp_str_slice[2]
		seconds_int, _ = strconv.Atoi(seconds_str)
	} else if len(temp_str_slice) == 2 {
		minutes_str = temp_str_slice[0]
		minutes_int, _ = strconv.Atoi(minutes_str)
		seconds_str = temp_str_slice[1]
		seconds_int, _ = strconv.Atoi(seconds_str)
	} else if len(temp_str_slice) == 1 {
		seconds_str = temp_str_slice[0]
		seconds_int, _ = strconv.Atoi(seconds_str)
	} else if len(temp_str_slice) == 0 {
		error_happened = "Could not interpret file split values"
	}

	if len(error_happened) == 0 {
		seconds_total_int = (hours_int * 60 * 60) + (minutes_int * 60) + seconds_int
		seconds_total_str = strconv.Itoa(seconds_total_int)

		if milliseconds_str != "" {
			seconds_total_str = seconds_total_str + "." + milliseconds_str
		}
	}

	return seconds_total_str, error_happened
}

func convert_cut_positions_to_timecode(cut_positions_after_processing_seconds []string) []string {

	var cut_positions_as_timecodes []string

	for counter, item := range cut_positions_after_processing_seconds {

		// Remove the first edit point if it is zero, as this really is no edit point
		if counter == 0 && item == "0" {
			continue
		}

		timecode := convert_seconds_to_timecode(item)

		cut_positions_as_timecodes = append(cut_positions_as_timecodes, timecode)
	}

	return cut_positions_as_timecodes
}

func convert_seconds_to_timecode(item string) string {

	item_str := item
	milliseconds_str := ""

	if strings.ContainsAny(item, ".") {
		item_str = strings.Split(item, ".")[0]
		milliseconds_str = strings.Split(item, ".")[1]
	}

	item_int, _ := strconv.Atoi(item_str)
	hours_int := 0
	minutes_int := 0
	seconds_int := 0
	timecode := ""

	if item_int/3600 > 0 {
		hours_int = item_int / 3600
		item_int = item_int - (hours_int * 3600)
	}

	if item_int/60 > 0 {
		minutes_int = item_int / 60
		item_int = item_int - (minutes_int * 60)
	}
	seconds_int = item_int

	hours_str := strconv.Itoa(hours_int)

	if len(hours_str) < 2 {
		hours_str = "0" + hours_str
	}

	minutes_str := strconv.Itoa(minutes_int)

	if len(minutes_str) < 2 {
		minutes_str = "0" + minutes_str
	}

	seconds_str := strconv.Itoa(seconds_int)

	if len(seconds_str) < 2 {
		seconds_str = "0" + seconds_str
	}

	timecode = hours_str + ":" + minutes_str + ":" + seconds_str

	if len(milliseconds_str) > 0 {
		timecode = timecode + "." + milliseconds_str
	}

	return timecode
}

func process_split_times(split_times string, debug_option bool) ([]string, []string) {

	var cut_list_string_slice, cut_list_seconds_str_slice, cut_list_positions_and_durations_seconds, cut_positions_after_processing_seconds, cut_positions_as_timecodes []string
	var seconds_total_str, error_happened string

	var runes []rune

	// The time values have characters 0-9 and : and . in them. Recognize these characters as time values.
	// This lets the user use any other character to separate time values from each other,
	// excluding characters that the shell tries to interpret: ()#!<>*/
	for _, item := range split_times {

		switch item {
		case rune('0'),rune('1'),rune('2'),rune('3'),rune('4'),rune('5'),rune('6'),rune('7'),rune('8'),rune('9'),rune(':'),rune('.'):
			runes = append(runes, item)
		default:
			cut_list_string_slice = append(cut_list_string_slice, string(runes))
			runes = nil
		}
	}

	cut_list_string_slice = append(cut_list_string_slice, string(runes)) // Append the last time value to the slice

	if len(cut_list_string_slice)%2 != 0 {
		fmt.Println("\nError: Split timecodes must be given in pairs (start_time, stop_time). There are:", len(cut_list_string_slice), "times on the commandline\n")
		os.Exit(1)
	}

	//////////////////////////////////////////////////////////////
	// Convert time values (01:20:25) to seconds (4825 seconds) //
	//////////////////////////////////////////////////////////////
	for _, temp_string := range cut_list_string_slice {

		if strings.ToLower(temp_string) == "start" {
			temp_string = "0"
		}

		if strings.ToLower(temp_string) == "end" {
			cut_list_seconds_str_slice = append(cut_list_seconds_str_slice, strings.ToLower(temp_string))
			break
		}

		seconds_total_str, error_happened = convert_timecode_to_seconds(temp_string)

		if error_happened != "" {
			fmt.Println("\nError when converting times to seconds: " + error_happened + "\n")
			os.Exit(1)
		}
		cut_list_seconds_str_slice = append(cut_list_seconds_str_slice, seconds_total_str)
	}

	///////////////////////////////////////////////////////////
	// Test that all times are ascending and not overlapping //
	///////////////////////////////////////////////////////////

	var previous_item string

	if debug_option == true {
		fmt.Println("")
		fmt.Println("process_split_times: cut_list_seconds_str_slice:", cut_list_seconds_str_slice)
	}

	for _, current_item := range cut_list_seconds_str_slice {

		if current_item == "end" {
			break
		}
		_, remaining_float := custom_float_substraction(current_item, previous_item)

		if remaining_float < 0.0 {
			var temp_str_slice []string
			temp_str_slice = append(temp_str_slice, previous_item, current_item)
			temp_2_str_slice := convert_cut_positions_to_timecode(temp_str_slice)

			if debug_option == true {
				fmt.Println("process_split_times: temp_str_slice:", temp_str_slice, "process_split_times: temp_2_str_slice:", temp_2_str_slice)
			}

			fmt.Println("\nError: times " + temp_2_str_slice[0] + " and " + temp_2_str_slice[1] + " are not in ascending order. Timecodes must be ascending and not overlap\n")
			os.Exit(1)
		}
		previous_item = current_item
	}

	///////////////////////////////////////////////////////////////////////////////////////////
	// Convert odd time values to duration. Even values are start times and used as they are //
	///////////////////////////////////////////////////////////////////////////////////////////

	for counter := 0; counter < len(cut_list_seconds_str_slice); counter = counter + 2 {

		start_time_string := ""
		stop_time_string := ""

		// Store the start time as it is
		cut_list_positions_and_durations_seconds = append(cut_list_positions_and_durations_seconds, cut_list_seconds_str_slice[counter])
		start_time_string = cut_list_seconds_str_slice[counter]

		if len(cut_list_seconds_str_slice)-1 > counter {
			stop_time_string = cut_list_seconds_str_slice[counter+1]

		}

		// If word 'end' is used to mark the end of file, then remove it, FFmpeg automatically processes to the end of file if the last duration is left out
		if strings.ToLower(stop_time_string) == "end" {
			break
		}

		duration_str, remaining_float := custom_float_substraction(stop_time_string, start_time_string)

		if remaining_float < 0.0 {
			fmt.Println("\nError: Stop time:", stop_time_string, "cannot be less than start time:", start_time_string)
			fmt.Println("All times must be absolute timecode positions NOT start times and durations\n")
			os.Exit(1)
		}

		// Store the duration
		cut_list_positions_and_durations_seconds = append(cut_list_positions_and_durations_seconds, duration_str)
	}

	//////////////////////////////////////////////////////////////////////////////////////////////////
	// Calculate where edit points are in the processed file so that the user can check them easily //
	//////////////////////////////////////////////////////////////////////////////////////////////////

	if len(cut_list_seconds_str_slice) > 2 {
		duration_of_a_used_file_part_str := ""
		duration_of_all_used_file_parts_str := ""
		duration_of_all_removed_file_parts_str := ""
		duration_of_a_removed_file_part_str := ""
		previous_stop_time_str := "0"

		for counter := 0; counter < len(cut_list_seconds_str_slice); counter = counter + 2 {
			start_time_str := cut_list_seconds_str_slice[counter]

			duration_of_a_removed_file_part_str, _ = custom_float_substraction(start_time_str, previous_stop_time_str)
			duration_of_all_removed_file_parts_str = custom_float_addition(duration_of_all_removed_file_parts_str, duration_of_a_removed_file_part_str)

			if counter+1 < len(cut_list_seconds_str_slice) {
				stop_time_str := cut_list_seconds_str_slice[counter+1]

				new_edit_position, _ := custom_float_substraction(start_time_str, duration_of_all_removed_file_parts_str)
				cut_positions_after_processing_seconds = append(cut_positions_after_processing_seconds, new_edit_position)
				previous_stop_time_str = stop_time_str

				// If word 'end' is used to mark the end of file, then remove it, FFmpeg automatically process to the end of file if the last duration is left out
				if strings.ToLower(stop_time_str) == "end" {
					break
				}

				duration_of_a_used_file_part_str, _ = custom_float_substraction(stop_time_str, start_time_str)
				duration_of_all_used_file_parts_str = custom_float_addition(duration_of_all_used_file_parts_str, duration_of_a_used_file_part_str)
			}
		}
	}

	// Convert seconds to timecode values
	cut_positions_as_timecodes = convert_cut_positions_to_timecode(cut_positions_after_processing_seconds)

	if debug_option == true {
		fmt.Println("process_split_times: split_times:", split_times)
		fmt.Println("process_split_times: cut_list_positions_and_durations_seconds:", cut_list_positions_and_durations_seconds)
		fmt.Println("process_split_times: cut_positions_after_processing_seconds:", cut_positions_after_processing_seconds)
		fmt.Println("process_split_times: cut_positions_as_timecodes:", cut_positions_as_timecodes)
	}

	return cut_list_positions_and_durations_seconds, cut_positions_as_timecodes
}

func custom_float_addition(value_1_str string, value_2_str string) (remaining_str string) {

	// Add two floats losslessly without using the unprecise float type
	var value_1_whole_int, value_1_fractions_int, value_2_whole_int, value_2_fractions_int, remaining_int, remaining_milliseconds_int int
	var value_1_fractions_str, value_2_fractions_str string
	var remaining_float float64

	temp_1_str := strings.Split(value_1_str, ".")
	value_1_whole_str := temp_1_str[0]

	if len(temp_1_str) > 1 {
		value_1_fractions_str = temp_1_str[1]
	}

	value_1_whole_int, _ = strconv.Atoi(value_1_whole_str)

	// If user gave a value .8 covert it to .800
	if len(value_1_fractions_str) > 1 {
		for len(value_1_fractions_str) < 3 {
			value_1_fractions_str = value_1_fractions_str + "0"
		}
	}

	value_1_fractions_int, _ = strconv.Atoi(value_1_fractions_str)

	temp_2_str := strings.Split(value_2_str, ".")
	value_2_whole_str := temp_2_str[0]

	if len(temp_2_str) > 1 {
		value_2_fractions_str = temp_2_str[1]
	}

	value_2_whole_int, _ = strconv.Atoi(value_2_whole_str)

	if len(value_2_fractions_str) > 1 {
		for len(value_2_fractions_str) < 3 {
			value_2_fractions_str = value_2_fractions_str + "0"
		}
	}

	value_2_fractions_int, _ = strconv.Atoi(value_2_fractions_str)

	remaining_int = value_1_whole_int + value_2_whole_int
	remaining_milliseconds_int = value_1_fractions_int + value_2_fractions_int

	// Add 1000 milliseconds from the whole numbers
	if remaining_milliseconds_int >= 1000 {
		remaining_milliseconds_int = remaining_milliseconds_int - 1000
		remaining_int++
	}

	remaining_str = strconv.Itoa(remaining_int)

	if remaining_milliseconds_int > 0 {
		remaining_milliseconds_str := strconv.Itoa(remaining_milliseconds_int)

		// Fill the start of the milliseconds string with zeroes
		for len(remaining_milliseconds_str) < 3 {
			remaining_milliseconds_str = "0" + remaining_milliseconds_str
		}

		remaining_str = remaining_str + "." + remaining_milliseconds_str
	}

	remaining_float, _ = strconv.ParseFloat(remaining_str, 64)

	if remaining_float < 0.0 {
		fmt.Println("\nError: Time addition rolled over and produced a negative number:", remaining_str, "\n")
		os.Exit(1)
	}

	return remaining_str
}

func custom_float_substraction(value_1_str string, value_2_str string) (remaining_str string, remaining_float float64) {

	// Subtract two floats losslessly without using the unprecise float type
	// The first value (value_1_str) needs to be the bigger one, since we subtract the second from the first
	var value_1_whole_int, value_1_fractions_int, value_2_whole_int, value_2_fractions_int, remaining_int, remaining_milliseconds_int int
	var value_1_fractions_str, value_2_fractions_str string

	temp_1_str := strings.Split(value_1_str, ".")
	value_1_whole_str := temp_1_str[0]

	if len(temp_1_str) > 1 {
		value_1_fractions_str = temp_1_str[1]
	}

	value_1_whole_int, _ = strconv.Atoi(value_1_whole_str)

	// If user gave a value .8 covert it to .800
	if len(value_1_fractions_str) > 1 {
		for len(value_1_fractions_str) < 3 {
			value_1_fractions_str = value_1_fractions_str + "0"
		}
	}

	value_1_fractions_int, _ = strconv.Atoi(value_1_fractions_str)

	temp_2_str := strings.Split(value_2_str, ".")
	value_2_whole_str := temp_2_str[0]

	if len(temp_2_str) > 1 {
		value_2_fractions_str = temp_2_str[1]
	}

	value_2_whole_int, _ = strconv.Atoi(value_2_whole_str)

	if len(value_2_fractions_str) > 1 {
		for len(value_2_fractions_str) < 3 {
			value_2_fractions_str = value_2_fractions_str + "0"
		}
	}

	value_2_fractions_int, _ = strconv.Atoi(value_2_fractions_str)

	// Borrow 1000 milliseconds from the whole numbers
	if value_2_fractions_int > value_1_fractions_int {
		value_1_fractions_int = value_1_fractions_int + 1000
		value_1_whole_int--
	}

	remaining_int = value_1_whole_int - value_2_whole_int
	remaining_milliseconds_int = value_1_fractions_int - value_2_fractions_int
	remaining_str = strconv.Itoa(remaining_int)

	if remaining_milliseconds_int > 0 {
		remaining_milliseconds_str := strconv.Itoa(remaining_milliseconds_int)

		// Fill the start of the milliseconds string with zeroes
		for len(remaining_milliseconds_str) < 3 {
			remaining_milliseconds_str = "0" + remaining_milliseconds_str
		}

		remaining_str = remaining_str + "." + remaining_milliseconds_str
	}

	remaining_float, _ = strconv.ParseFloat(remaining_str, 64)

	return remaining_str, remaining_float
}

func read_filenames_in_a_dir(source_dir string) (files_str_slice []string) {

	files, err := ioutil.ReadDir(source_dir)

	if err != nil {
		log.Fatal(err)
	}

	for _, entry := range files {

		if entry.IsDir() == false {
			files_str_slice = append(files_str_slice, entry.Name())
		}
	}

	return files_str_slice
}

func subtitle_trim(original_subtitles_absolute_path string, fixed_subtitles_absolute_path string, files_str_slice []string, video_width string, video_height string, process_number int, return_channel chan int, subtitle_burn_resize string, subtitle_burn_grayscale bool) {

	var subtitle_dimension_info []string
	var subtitle_resize_info []string
	var subtitles_dimension_map = make(map[string][]string)
	var subtitle_resize_commandline []string
	var subtitle_trim_commandline []string

	///////////////////////////////////////////////////////////////////
	// Trim subtitles, removing empty space around the subtitle text //
	///////////////////////////////////////////////////////////////////

	for _, subtitle_name := range files_str_slice {

		subtitle_trim_commandline = nil

		if subtitle_burn_grayscale == true {
			// Convert subtitle to grayscale and trim
			subtitle_trim_commandline = append(subtitle_trim_commandline, "magick", filepath.Join(original_subtitles_absolute_path, subtitle_name), "-trim", "-print", "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]", "-compress", "rle", "-set", "colorspace", "Gray", filepath.Join(fixed_subtitles_absolute_path, subtitle_name))
		} else {
			// Trim subtitle
			subtitle_trim_commandline = append(subtitle_trim_commandline, "magick", filepath.Join(original_subtitles_absolute_path, subtitle_name), "-trim", "-print", "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]", "-compress", "rle", filepath.Join(fixed_subtitles_absolute_path, subtitle_name))
		}

		subtitle_trim_output, subtitle_trim_error, trim_error_code := run_external_command(subtitle_trim_commandline)

		///////////////////////////////////////////////////////////////////////////////////////////////////
		// If there is no subtitle in the image, then create a subtitle file with an empty alpha channel //
		///////////////////////////////////////////////////////////////////////////////////////////////////
		if trim_error_code != nil {

			fmt.Println()
			fmt.Println("ImageMagick trim reported error: ", subtitle_trim_error)
			fmt.Println()

			continue
		}

		subtitle_resize_commandline = nil

		subtitle_resize_commandline = append(subtitle_resize_commandline, "mogrify", "+distort", "SRT", subtitle_burn_resize + ",0", "+repage", "-print", "%[fx:w],%[fx:h]", "-compress", "rle", filepath.Join(fixed_subtitles_absolute_path, subtitle_name))
		subtitle_resize_output, subtitle_resize_error, resize_error_code := run_external_command(subtitle_resize_commandline)

		if resize_error_code != nil {
			fmt.Println("Subtitle resize reported error:", subtitle_resize_error)
		}

		// Take image properties before and after crop and store them in a map.
		//
		// Image info in 'subtitle_dimension_info' is:
		// Original width before crop (not used at the moment)
		// Original height before crop (not used at the moment)
		// Cropped width
		// Cropped height
		// Start of crop on x axis
		// Start of crop on y axis (not used at the moment)
		// Subtitle width after resize
		// Subtitle height after resize

		subtitle_dimension_info = strings.Split(subtitle_trim_output[0], ",")

		if subtitle_burn_resize != "" {

			subtitle_resize_info = strings.Split(subtitle_resize_output[0], ",")
			subtitle_dimension_info = append(subtitle_dimension_info, subtitle_resize_info...)

		} else {

			subtitle_dimension_info = append(subtitle_dimension_info, "0", "0")
		}

		subtitles_dimension_map[subtitle_name] = subtitle_dimension_info
	}

	/////////////////////////////////////////////////////////////////////////
	// Overlay cropped subtitles on a new position on a transparent canvas //
	/////////////////////////////////////////////////////////////////////////

	video_height_int, _ := strconv.Atoi(video_height)
	video_width_int, _ := strconv.Atoi(video_width)
	var subtitle_adjust_commandline []string
	var subtitle_new_y int
	counter := 0

	// Define the position of the subtitle to be 5 - 20 pixels from the top / bottom of picture depending on the video height.
	var subtitle_margin int = video_height_int / 100

	if subtitle_margin < 5 {
		subtitle_margin = 5
	}

	if subtitle_margin > 20 {
		subtitle_margin = 20
	}

	for subtitle_name := range subtitles_dimension_map {

		counter++
		// orig_width ,_ := strconv.Atoi(subtitles_dimension_map[subtitle_name][0])
		// orig_height ,_:= strconv.Atoi(subtitles_dimension_map[subtitle_name][1])
		cropped_width, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][2])
		cropped_height, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][3])
		// cropped_start_x ,_:= strconv.Atoi(subtitles_dimension_map[subtitle_name][4])
		cropped_start_y, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][5])

		if subtitle_burn_resize != "" {
			cropped_width, _ = strconv.Atoi(subtitles_dimension_map[subtitle_name][6])
			cropped_height, _ = strconv.Atoi(subtitles_dimension_map[subtitle_name][7])
		}

		picture_center := video_height_int / 2 // Divider to find out if the subtitle is located above or below this line at the center of the picture
		subtitle_new_x := (video_width_int / 2) - (cropped_width / 2) // This centers cropped subtitle on the x axis

		if cropped_start_y > picture_center {
			// Center subtitle on the bottom of the picure
			subtitle_new_y = video_height_int - cropped_height - subtitle_margin

		} else {
			// Center subtitle on top of the picture
			subtitle_new_y = subtitle_margin
		}

		subtitle_adjust_commandline = nil
		subtitle_adjust_commandline = append(subtitle_adjust_commandline, "magick", "-size", video_width + "x" + video_height, "canvas:transparent", filepath.Join(fixed_subtitles_absolute_path, subtitle_name), "-geometry", "+" + strconv.Itoa(subtitle_new_x) + "+" + strconv.Itoa(subtitle_new_y), "-composite", "-compose", "over", "-compress", "rle", filepath.Join(fixed_subtitles_absolute_path, subtitle_name))

		_, subtitle_trim_error, error_code := run_external_command(subtitle_adjust_commandline)

		if error_code != nil {
			fmt.Println("Repositioning subtitle generated an error:", subtitle_trim_error)
		}
	}
	return_channel <- process_number
}

func get_number_of_physical_processors () (int, error) {

	/////////////////////////////////
	// This is Linux specific code //
	/////////////////////////////////

	last_physical_id_int := -1
	physical_id_int := -1
	physical_id_found := false
	cpu_cores_int := 0

	// Read in /proc/cpuinfo
	file_handle, err := os.Open("/proc/cpuinfo")

	if err != nil {
		return 0, err
	}

	defer file_handle.Close()

	scanner := bufio.NewScanner(file_handle)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {

		if strings.HasPrefix(scanner.Text(), "physical id") {
			temp_list  := strings.Split(scanner.Text(), ":")
			physical_id_int, err = strconv.Atoi(strings.TrimSpace(temp_list[1]))

			if physical_id_int != last_physical_id_int {
				physical_id_found = true
				last_physical_id_int = physical_id_int
				continue
			}
		}

		if err != nil {
			break
		}

		if physical_id_found == true && strings.HasPrefix(scanner.Text(), "cpu cores") {
			temp_int := -1
			temp_list  := strings.Split(scanner.Text(), ":")
			temp_int, err = strconv.Atoi(strings.TrimSpace(temp_list[1]))
			cpu_cores_int = cpu_cores_int + temp_int
			physical_id_found = false
		}

		if err != nil {
			break
		}
	}

	return cpu_cores_int, err
}

func remove_duplicate_subtitle_images (original_subtitles_absolute_path string, fixed_subtitles_absolute_path string, files_str_slice []string, video_width string, video_height string) (files_remaining []string) {

	var subtitle_md5sum_map  = make(map[string][]string)
	var subtitle_copies []string

	// Calculate md5 for each file
	for _, subtitle_name := range files_str_slice {

		subtitle_path := filepath.Join(original_subtitles_absolute_path, subtitle_name)
		filehandle, err := os.Open(subtitle_path)

		if err != nil {
			log.Fatal(err)
		}

		md5_handler := md5.New()

		if _, err := io.Copy(md5_handler, filehandle); err != nil {
			log.Fatal(err)
		}

		// Caculate md5 for the subtitle file
		md5sum := fmt.Sprintf("%x", md5_handler.Sum(nil))
		filehandle.Close()

		// If we have not stored this md5 before then store it and the name of the picture to map.
		// If we have stored the md5 before, add the picture name to list for this md5.
		if _, val := subtitle_md5sum_map[md5sum] ; val == false {
			subtitle_copies = nil
			subtitle_copies = append(subtitle_copies, subtitle_name)
			subtitle_md5sum_map[md5sum] = subtitle_copies

		} else {
			subtitle_copies = nil
			subtitle_copies = subtitle_md5sum_map[md5sum]
			subtitle_copies = append(subtitle_copies, subtitle_name)
			subtitle_md5sum_map[md5sum] = subtitle_copies
		}
	}

	// Trim images until we find one where there is no subtitle.
	// Create temp directory for trimmed images
	var empty_subtitle_creation_commandline_start []string
	empty_subtitle_creation_commandline_start = append(empty_subtitle_creation_commandline_start, "magick", "-size", video_width + "x" + video_height, "canvas:transparent", "-alpha", "on", "-compress", "rle")
	var empty_subtitle_creation_commandline []string

	var subtitle_trim_commandline []string
	var empty_subtitle_path string
	var empty_subtitle_md5 string

	temp_path := filepath.Join(original_subtitles_absolute_path, "00-temp_path")

	if _, err := os.Stat(temp_path); os.IsNotExist(err) {
		os.MkdirAll(temp_path, 0777)
	}

	for _, subtitle_name := range files_str_slice {

		subtitle_trim_commandline = nil
		subtitle_trim_commandline = append(subtitle_trim_commandline, "magick", filepath.Join(original_subtitles_absolute_path, subtitle_name), "-trim", "-print", "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]", "-compress", "rle", filepath.Join(temp_path, subtitle_name))
		_, subtitle_trim_error, trim_error_code := run_external_command(subtitle_trim_commandline)

		///////////////////////////////////////////////////////////////////////////////////////////////////
		// If there is no subtitle in the image, then create a subtitle file with an empty alpha channel //
		///////////////////////////////////////////////////////////////////////////////////////////////////
		if trim_error_code != nil {

			// Get md5 of the empty image
			subtitle_path := filepath.Join(original_subtitles_absolute_path, subtitle_name)
			filehandle, err := os.Open(subtitle_path)

			if err != nil {
				log.Fatal(err)
			}

			defer filehandle.Close()

			md5_handler := md5.New()

			if _, err := io.Copy(md5_handler, filehandle); err != nil {
				log.Fatal(err)
			}

			// Caculate md5 for the subtitle file
			empty_subtitle_md5 = fmt.Sprintf("%x", md5_handler.Sum(nil))

			// Create an empty picture with nothing but transparency in it and write it overwrinting the original.
			// This is needed to get this image and the ones later manipulated with ImageMagick to have the same bit depth and other properties.
			empty_subtitle_path = filepath.Join(fixed_subtitles_absolute_path, subtitle_name)
			empty_subtitle_creation_commandline = append(empty_subtitle_creation_commandline_start, empty_subtitle_path)
			_, _, error_code := run_external_command(empty_subtitle_creation_commandline)

			if error_code != nil {
				fmt.Println("\n\nCreating an empty subtitle image generated an error:", subtitle_trim_error)
			}

			break // Jump out of the loop when the first image without subtitle has been found
		}
	}

	// Create soft links for empty image duplicates
	var new_empty_subtitle string
	subtitle_copies = nil
	subtitle_copies = subtitle_md5sum_map[empty_subtitle_md5]

	for counter, filename := range subtitle_copies {

		if counter == 0 {
			new_empty_subtitle = filename
			continue
		}

		err := os.Symlink(filepath.Join(fixed_subtitles_absolute_path, new_empty_subtitle), filepath.Join(fixed_subtitles_absolute_path, filename))

		if err != nil {
			log.Fatal(err)
		}

	}
	delete (subtitle_md5sum_map, empty_subtitle_md5)

	// Create soft links for the rest of subtitle image duplicates
	for _, subtitle_copies := range subtitle_md5sum_map {

		for counter, filename := range subtitle_copies {

			if counter == 0 {
				new_empty_subtitle = filename
				files_remaining = append(files_remaining, filename)
				continue
			}

			err := os.Symlink(filepath.Join(fixed_subtitles_absolute_path, new_empty_subtitle), filepath.Join(fixed_subtitles_absolute_path, filename))

			if err != nil {
				log.Fatal(err)
			}
		}
	}

	return files_remaining
}

func store_options_and_help_text_int(category string, option string, value int, help_text string) *commandline_struct {

	// This function does what flag.Int does in go and extends it a little
	// The function stores category, option name, variables and help text for a commandline option.
	// It also returns pointer to the value default value for the commandline option
	// so that it can be assigned to a variable.

	if category == "" {
		fmt.Println("Error, help_text category is empty")
		os.Exit(0)
	}

	if option == "" {
		fmt.Println("Error, help_text option is empty")
		os.Exit(0)
	}

	if help_text == "" {
		fmt.Println("Error, help_text message is empty")
		os.Exit(0)
	}

	commandline_option_struct := new(commandline_struct)
	commandline_option_struct.option_type = "int"
	commandline_option_struct.user_int = value
	commandline_option_struct.help_text = help_text

	// Store options for each category
	if len(helptext_categories_map) == 0 {
		// The list of options is empty, store the first option in the category
		var temp_slice []string
		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	} else {
		// There already are some options in the list, store the new one at the end of the list
		var temp_slice = helptext_categories_map[category]

		for _, item := range temp_slice {
			if option == item {
				fmt.Println("Error, option defined twice")
				os.Exit(0)
			}
		}

		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	}

	// Store helptext for the option
	commandline_option_map[option] = commandline_option_struct

	return commandline_option_struct
}

func store_options_and_help_text_bool(category string, option string, help_text string) *commandline_struct {

	// This function does what flag.Bool does in go and extends it a little
	// The function stores category, option name, variables and help text for a commandline option.
	// It also returns pointer to the value default value for the commandline option
	// so that it can be assigned to a variable.

	if category == "" {
		fmt.Println("Error, help_text category is empty")
		os.Exit(0)
	}

	if option == "" {
		fmt.Println("Error, help_text option is empty")
		os.Exit(0)
	}

	if help_text == "" {
		fmt.Println("Error, help_text message is empty")
		os.Exit(0)
	}

	commandline_option_struct := new(commandline_struct)
	commandline_option_struct.option_type = "bool"
	commandline_option_struct.help_text = help_text

	// Store options for each category
	if len(helptext_categories_map) == 0 {
		// The list of options is empty, store the first option in the category
		var temp_slice []string
		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	} else {
		// There already are some options in the list, store the new one at the end of the list
		var temp_slice = helptext_categories_map[category]

		for _, item := range temp_slice {
			if option == item {
				fmt.Println("Error, option defined twice")
				os.Exit(0)
			}
		}

		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	}

	// Store helptext for the option
	commandline_option_map[option] = commandline_option_struct

	return commandline_option_struct
}

func store_options_and_help_text_string(category string, option string, value string, help_text string) *commandline_struct {

	// This function does what flag.String does in go and extends it a little
	// The function stores category, option name, variables and help text for a commandline option.
	// It also returns pointer to the value default value for the commandline option
	// so that it can be assigned to a variable.

	if category == "" {
		fmt.Println("Error, help_text category is empty")
		os.Exit(0)
	}

	if option == "" {
		fmt.Println("Error, help_text option is empty")
		os.Exit(0)
	}

	if help_text == "" {
		fmt.Println("Error, help_text message is empty")
		os.Exit(0)
	}

	commandline_option_struct := new(commandline_struct)
	commandline_option_struct.option_type = "string"
	commandline_option_struct.user_string = value
	commandline_option_struct.help_text = help_text

	// Store options for each category
	if len(helptext_categories_map) == 0 {
		// The list of options is empty, store the first option in the category
		var temp_slice []string
		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	} else {
		// There already are some options in the list, store the new one at the end of the list
		var temp_slice = helptext_categories_map[category]

		for _, item := range temp_slice {
			if option == item {
				fmt.Println("Error, option defined twice")
				os.Exit(0)
			}
		}

		temp_slice = append(temp_slice, option)
		helptext_categories_map[category] = temp_slice
	}

	// Store helptext for the option
	commandline_option_map[option] = commandline_option_struct

	return commandline_option_struct
}

func display_help_text() {

	// Get terminal window dimensions
	value_slice_int := get_terminal_window_dimensions()
	// Don't print at the last character on the line because terminal will then automatically insert a line feed messing our print output.
	terminal_width_int := value_slice_int[0] - 1
	// terminal_height_int := value_slice_int[1] // We don't use terminal height on anything at this point

	var second_paragraph_start, helptext_start, helptext_end int
	var textline string

	// Get map keys and sort them
	categories := make ([]string, 0, len(helptext_categories_map))

	for category := range helptext_categories_map {
		categories = append(categories, category)
	}

	sort.Strings(categories)

	///////////////////////////////////////////////////////////
	// Find the longest option and adjust help text printing //
	// start point in reference to that                      //
	///////////////////////////////////////////////////////////
	for _, category := range categories {
		option_list := helptext_categories_map[category]

		for _, option := range option_list {
			if len(option) > second_paragraph_start {
				second_paragraph_start = len(option)
			}
		}
	}

	second_paragraph_start = second_paragraph_start + 4

	/////////////////////////////////////////////////////////
	// Print sorted categories and options in each of them //
	/////////////////////////////////////////////////////////
	for _, category := range categories {
		option_list := helptext_categories_map[category]

		fmt.Println()
		fmt.Println(category + ":")
		fmt.Println(strings.Repeat("-", len(category) + 1))

		for _, option := range option_list {

			///////////////////////////////////////
			// Print the first line of help text //
			///////////////////////////////////////
			commandline_option_struct := commandline_option_map[option]
			textline = option + strings.Repeat(" ", second_paragraph_start - len(option))
			helptext_start = 0
			helptext_end = terminal_width_int - len(textline)

			if helptext_end > len(commandline_option_struct.help_text) {
				helptext_end = len(commandline_option_struct.help_text)
			}

			// Make sure we wont cut the last word in the middle at the end of the line
			if helptext_end > 0 && helptext_end < len(commandline_option_struct.help_text) {

				for commandline_option_struct.help_text[helptext_end - 1] != ' ' {
					helptext_end--

					if helptext_end <= 0 {
						helptext_end = 0
						break
					}
				}
			}

			fmt.Println("-" + option + strings.Repeat(" ", second_paragraph_start - len(option)) + commandline_option_struct.help_text[helptext_start:helptext_end])

			// If this was the last line, start printing the next option help text
			if helptext_end >= len(commandline_option_struct.help_text) {
				fmt.Println()
				continue
			}

			//////////////////////////////////////////////////////
			// Print second help text line and lines after that //
			//////////////////////////////////////////////////////
			if helptext_end < len(commandline_option_struct.help_text) {

				for {
					helptext_start = helptext_end
					helptext_end = helptext_start + terminal_width_int - second_paragraph_start

					if helptext_end > len(commandline_option_struct.help_text) {
						helptext_end = len(commandline_option_struct.help_text)
					}

					// If the first character on the line is a space then skip it
					// and start from the first character of the next word instead
					if commandline_option_struct.help_text[helptext_start] == ' ' {
						helptext_start++

						if helptext_start > len(commandline_option_struct.help_text) {
							helptext_start = len(commandline_option_struct.help_text)
						}
					}

					// Make sure we wont cut the last word in the middle at the end of the line
					if helptext_end < len(commandline_option_struct.help_text) {

						for commandline_option_struct.help_text[helptext_end] != ' ' {
						helptext_end--

							if helptext_end <= 0 {
								helptext_end = 0
								break
							}
						}
					}

					fmt.Println(strings.Repeat(" ", second_paragraph_start), commandline_option_struct.help_text[helptext_start:helptext_end])

					// If this was the last line, start printing the next option help text
					if helptext_end >= len(commandline_option_struct.help_text) {
						break
					}
				}
			}
			fmt.Println()
		}
	}
	os.Exit(0)
}

func get_terminal_window_dimensions() []int {

	var value_slice_int []int

	// Get terminal window size with the stty - command, if it does not exist return 1000,1000
	if _, error := exec.LookPath("stty"); error == nil {

		cmd := exec.Command("stty", "size")
		cmd.Stdin = os.Stdin
		out, err := cmd.Output()

		if err != nil {
			fmt.Println("Error getting terminal window width and height")
			os.Exit(0)                                                                                                                                                                                                                     
		}

		first_line := strings.Split(string(out), "\n")
		values_slice := strings.Split(first_line[0], " ")

		height_string := values_slice[0]
		width_string := values_slice[1]

		height_int, error_happened := strconv.Atoi(height_string)

		if error_happened != nil {                                                                                                                                                                                                             
			fmt.Println("Error converting terminal window height to int")
			os.Exit(0)                                                                                                                                                                                                                     
		} 

		width_int, error_happened := strconv.Atoi(width_string)

		if error_happened != nil {                                                                                                                                                                                                             
			fmt.Println("Error converting terminal window width to int")
			os.Exit(0)                                                                                                                                                                                                                     
		} 

		value_slice_int = append(value_slice_int, width_int)
		value_slice_int = append (value_slice_int, height_int)
	} else {
		value_slice_int = append(value_slice_int, 1000)
		value_slice_int = append(value_slice_int, 1000)
	}

	return value_slice_int
}

func parse_options() []string {

	// Debug mode for this subroutine cannot be set on the commandline
	// because we have not parsed the commandline when we enter here
	// Instead turn debug to true in the debug variable.

	///////////////////////////////
	// Parse commandline options //
	///////////////////////////////
	debug := false

	if debug == true {

		var arguments []string
		arguments = os.Args
		fmt.Println()
		fmt.Println("Arguments:", arguments)
		fmt.Println()

		fmt.Println("Argument slice items:")
		fmt.Println("---------------------")

		for counter, item := range arguments {
			fmt.Println(counter, item)
		}
		fmt.Println()
		fmt.Println("Parsed arguments")
		fmt.Println("----------------")
	}

	var input_filenames []string
	var commandline_option_struct *commandline_struct
	var predefined_option string
	var option_found, string_option_found, int_option_found, item_is_an_option bool
	string_option_found = false
	int_option_found = false

	for _, commandline_option := range os.Args[1:] {

		option_found = false
		item_is_an_option = false

		// Remove leading "-" characters from the option string
		// If the previous round in the for loop has recognized that the next
		// string on the commandline is not an option but and value for it
		// then don't remove the - character. This is because some 
		// options takes negative numbers as values.
		if string_option_found == false && int_option_found == false {

			var option_as_runes []rune

			if []rune(commandline_option)[0] == rune('-') {
				item_is_an_option = true

				for _, item := range commandline_option {

					if item != rune('-') {
						option_as_runes = append(option_as_runes, item)
					}
				}
				commandline_option = string(option_as_runes)
			}
		}

		// This part assigns int or string value following an option to the commandline variable 
		// After the assingment the next option is fetched from the commandline
		if string_option_found == true {
			commandline_option_struct.user_string = commandline_option
			string_option_found = false
			continue
		}

		if int_option_found == true {
			temp_int, error_happened := strconv.Atoi(commandline_option)

			if error_happened != nil {
				fmt.Println("Error could not convert option to integer:", commandline_option)
				os.Exit(0)
			}

			commandline_option_struct.user_int = temp_int

			int_option_found = false
			continue
		}

		// This part recognizes an option on the commandline and sets boolean values so that 
		// the int or string following the option is handled at the next round of the for loop
		if item_is_an_option == true {

			for predefined_option, commandline_option_struct = range commandline_option_map {

				if commandline_option == predefined_option {

					if commandline_option_struct.option_type == "int" {

						if debug == true {
							fmt.Print("found int variable:", commandline_option, " ")
						}

						commandline_option_struct.is_turned_on = true
						int_option_found = true
						option_found = true
						break
					}

					if commandline_option_struct.option_type == "bool" {

						if debug == true {
							fmt.Println("found bool variable", commandline_option, " ")
						}

						// Store value: true to this options struct and switch the variable connected
						// to this commandline option to point to the value in struct
						commandline_option_struct.is_turned_on = true
						option_found = true
						break
					}

					if commandline_option_struct.option_type == "string" {

						if debug == true {
							fmt.Print("found string variable:", commandline_option, " ")
						}

						commandline_option_struct.is_turned_on = true
						string_option_found = true
						option_found = true
						break
					}
				}
			}

			if option_found == false {

				fmt.Println()
				fmt.Println("Error, unknown option: -" + commandline_option)
				fmt.Println()

				os.Exit(0)
			}

		} else {

			// The item on the commandline is a filename, test if files exist
			inputfile_full_path,_ := filepath.Abs(commandline_option)
			fileinfo, err := os.Stat(inputfile_full_path)

			// Test if input files exist
			if os.IsNotExist(err) == true {

				fmt.Println()
				fmt.Println("Error !!!!!!!")
				fmt.Println("File: '" + inputfile_full_path + "' does not exist")
				fmt.Println()

				os.Exit(1)
			}

			// Test if name is a directory
			if fileinfo.IsDir() == true {

				fmt.Println()
				fmt.Println("Error !!!!!!!")
				fmt.Println(inputfile_full_path + " is not a file it is a directory.")
				fmt.Println()

				os.Exit(1)
			}

			// Add all existing input file names to a slice
			input_filenames = append(input_filenames, inputfile_full_path)
		}
	}

	if debug == true {

		// Print commandline options and their values in alphabetical order
		// Print only those options that the user has turned on
		var sorted_options []string
		var longest_option int
		var title_to_print string

		fmt.Println("\n")
		title_to_print = "All option variables and text:"
		fmt.Println(title_to_print)
		fmt.Println(strings.Repeat("-", len(title_to_print)))
		print_all_commandline_variables()

		fmt.Println("\n")
		title_to_print = "Options that the user turned on:"
		fmt.Println(title_to_print)
		fmt.Println(strings.Repeat("-", len(title_to_print)))

		for option := range commandline_option_map {
			sorted_options = append(sorted_options, option)

			if len(option) > longest_option {
				longest_option = len(option)
			}
		}

		sort.Strings(sorted_options)

		for _, option := range sorted_options {

			commandline_option_struct = commandline_option_map[option]

			if commandline_option_struct.is_turned_on == true {

				fmt.Printf("-" + option + strings.Repeat(" ", longest_option - len(option)))
				fmt.Printf("   " + commandline_option_struct.option_type + "   ")

				if commandline_option_struct.option_type == "int" {
					fmt.Println("     ", commandline_option_struct.user_int)
				} else if commandline_option_struct.option_type == "bool" {
					fmt.Println("     ", commandline_option_struct.is_turned_on)
				} else if commandline_option_struct.option_type == "string" {
					fmt.Println("   ", commandline_option_struct.user_string)
				}
			}
		}
	}

	return input_filenames
}

func print_all_commandline_variables() {

	// This subroutine is used to debug option variables and help text.
	for option, commandline_option_struct := range commandline_option_map {
		fmt.Println()
		fmt.Println("Option:", option)

		if commandline_option_struct.option_type == "bool" {
			fmt.Println("\tType:", commandline_option_struct.option_type)
			fmt.Println("\tValue:", commandline_option_struct.is_turned_on)
			fmt.Println("\tHelp text:", commandline_option_struct.help_text)
		}

		if commandline_option_struct.option_type == "int" {
			fmt.Println("\tType:", commandline_option_struct.option_type)
			fmt.Println("\tValue:", commandline_option_struct.user_int)
			fmt.Println("\tHelp text:", commandline_option_struct.help_text)
		}

		if commandline_option_struct.option_type == "string" {
			fmt.Println("\tType:", commandline_option_struct.option_type)
			fmt.Println("\tValue:", commandline_option_struct.user_string)
			fmt.Println("\tHelp text:", commandline_option_struct.help_text)
		}
	}
}

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	// Print executable name and version
	fmt.Println(filepath.Base(os.Args[0]), "version", version_number)

	////////////////////////////////
	// Define commandline options //
	////////////////////////////////
	// Below are variable, and helpt text definitions tied to a commandline option
	// Definitions from left to right are:
	// Variable that gets the default value defined later on the line
	// Category. Help text will be printed a category at a time
	// Commandline option name.
	// Default value for the variable. The function "store_options_and_help_text_string()" returns this value to the variable defined at the beginning of the line
	// Help text for the commandline option
	// Address of the variable defined at the beginning of the line. This is used when the commandline option is followed by a value. The variable address is used to point the variable to the user defined value.
	// Audio options
	audio_language_option := store_options_and_help_text_string("Audio", "a", "", "Select audio with this language code, example: -a fin or -a eng or -a ita  Only one audio stream can be selected. Only one of the options -an and -a can be used at the a time.")
	audio_stream_number_option := store_options_and_help_text_string("Audio", "an", "0", "Select audio stream by number, example: -an 1. Only one audio stream can be selected. Only one of the options -an and -a can be used at the a time.")
	audio_compression_ac3 := store_options_and_help_text_bool("Audio", "ac3", "Compress audio as ac3. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate. 6 channels uses the ac3 max bitrate of 640k.")
	audio_compression_aac := store_options_and_help_text_bool("Audio", "aac", "Compress audio as aac. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.", )
	audio_compression_opus := store_options_and_help_text_bool("Audio", "opus", "Compress audio as opus. Opus support in mp4 container is experimental as of FFmpeg vesion 4.2.1. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.")
	audio_compression_flac := store_options_and_help_text_bool("Audio", "flac", "Compress audio in lossless Flac - format")
	no_audio := store_options_and_help_text_bool("Audio", "na", "Disable audio processing. There is no audio in the resulting file.")

	// Video options
	autocrop_option := store_options_and_help_text_bool("Video", "ac", "Autocrop. Find crop values automatically by doing 10 second spot checks in 10 places for the duration of the file.")
	crf_option := store_options_and_help_text_bool("Video", "crf", "Use Constant Quality instead of 2-pass encoding. The default value for crf is 18, which produces the same quality as default 2-pass but a bigger file. CRF is much faster that 2-pass encoding.")
	denoise_option := store_options_and_help_text_bool("Video", "dn", "Denoise. Use HQDN3D - filter to remove noise from the picture. This option is equal to Hanbrakes 'medium' noise reduction settings.")
	grayscale_option := store_options_and_help_text_bool("Video", "gr", "Convert video to Grayscale. Use this option if the original source is black and white. This results more bitrate being available for b/w information and better picture quality.")
	inverse_telecine := store_options_and_help_text_bool("Video", "it", "Perform inverse telecine on 29.97 fps material to return it back to original 24 fps.")
	main_bitrate_option := store_options_and_help_text_string("Video", "mbr", "", "Override automatic bitrate calculation for main video and define bitrate manually.")
	no_deinterlace := store_options_and_help_text_bool("Video", "nd", "No Deinterlace. By default deinterlace is always used. This option disables it.")
	parallel_sd := store_options_and_help_text_bool("Video", "psd", "Parallel SD. Create SD version in parallel to HD processing. This creates an additional version of the video downconverted to SD resolution. The SD file is stored in directory: 00-processed_files/sd")
	sd_bitrate_option := store_options_and_help_text_string("Video", "sbr", "", "Override automatic bitrate calculation for parallely created sd video and define bitrate manually.")
	scale_to_sd := store_options_and_help_text_bool("Video", "ssd", "Scale to SD. Scale video down to SD resolution. Calculates resolution automatically. Video is stored in directory 'sd'")
	burn_timecode := store_options_and_help_text_bool("Video", "tc", "Burn timecode on top of video. Timecode can be used for example to look for exact edit points for the file split feature.")

	// Options that affect both video and audio
	force_lossless := store_options_and_help_text_bool("Audio and Video", "ls", "Force encoding to use lossless utvideo compression for video and flac compression for audio. This also turns on -fe (1-Pass encode). This option only affects the main video if used with the -psd option.")

	// Subtitle options
	subtitle_language_option := store_options_and_help_text_string("Subtitle", "s", "", "Burn subtitle with this language code on top of video. Example: -s fin or -s eng or -s ita  Only use option -sn or -s not both.")
	subtitle_burn_downscale := store_options_and_help_text_bool("Subtitle", "sd", "Subtitle `downscale`. When cropping video widthwise, scale subtitle down to fit on top of the cropped video. This results in a smaller subtitle font. The -sd option affects only subtitle burned on top of video.")
	subtitle_burn_grayscale := store_options_and_help_text_bool("Subtitle", "sgr", "Subtitle Grayscale. Remove color from subtitle by converting it to grayscale. This option only works with subtitle burned on top of video. If video playback is glitchy every time a subtitle is displayed, then removing color from subtitle may help.")
	subtitle_stream_number_option := store_options_and_help_text_string("Subtitle", "sn", "-1", "Burn subtitle with this stream number on top of video. Example: -sn 1. Only use option -sn or -s not both.")
	subtitle_vertical_offset := store_options_and_help_text_string("Subtitle", "so", "0", "Subtitle `offset`, -so 55 (move subtitle 55 pixels down), -so -55 (move subtitle 55 pixels up). This option affects only subtitle burned on top of video. Also check the -sp option that automatically moves subtitles near the edge of the screen.")
	subtitle_mux_language_option := store_options_and_help_text_string("Subtitle", "sm", "", "Mux subtitles with these language codes into the target file. Example: -sm eng or -sm eng,fra,fin. This only works with dvd, dvb and bluray bitmap based subtitles. Mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file using the -mkv option.")
	subtitle_mux_numbers_option := store_options_and_help_text_string("Subtitle", "smn", "", "Mux subtitles with these stream numbers into the target file. Example: -smn 1 or -smn 3,1,7. This only works with dvd, dvb and bluray bitmap based subtitles. Mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file using the -mkv option.")
	subtitle_burn_palette := store_options_and_help_text_string("Subtitle", "palette", "", "Hack the dvd subtitle color palette. The subtitle color palette defines the individual colors used in the subtitle (border, middle, etc). This option takes 1-16 comma separated hex numbers ranging from 0 to f. Zero = black, f = white, so only shades between black -> gray -> white can be defined. If you define less than the required 16 numbers then the rest will be filled with f's. Each dvd uses color mapping differently so you need to test which numbers control the colors you want to change. Usually the first 4 numbers control the colors. Example: -palette f,0,f . This option only affects subtitle burned on top of video.")
	subtitle_burn_split := store_options_and_help_text_bool("Subtitle", "sp", "Subtile Split. Subtitles on DVD's and Blurays often use an unnecessary large font and are positioned too far from the edge of the screen covering too much of the picture. Sometimes subtitles are also displayed on the upper part of the screen and may even cover the actors face. The -sp option detects whether the subtitle is displayed top or bottom half of the screen and then moves it towards that edge of the screen so that it covers less of the picture area. Distance from the screen edge is calculated automatically based on video resolution (picture height divided by 100 and rounded down to nearest integer. Minimum distance is 5 pixels and max 20 pixels). Subtitles are also automatically centered horizontally. Use the -sr option with -sp to resize subtitle. The -sp option affects only subtitles burned on top of video.")
	subtitle_burn_resize := store_options_and_help_text_string("Subtitle", "sr", "", "Subtitle Resize. Values less than 1 makes subtitles smaller, values bigger than 1 makes them larger. This option can only be used with the -sp option. Example: make subtitle 25% smaller: -sr 0.75   make subtitle 50% smaller: -sr 0.50 make subtitle 75% larger: -sr 1.75. This option affects only subtitle burned on top of video.")

	// Scan options
	fast_encode_and_search := store_options_and_help_text_bool("Scan", "f", "This is the same as using options -fs and -fe at the same time.")
	fast_encode := store_options_and_help_text_bool("Scan", "fe", "Fast encoding mode. Encode video using 1-pass encoding. Use this for testing to speed up processing. Video quality will be much lower than with 2-Pass encoding.")
	fast_search := store_options_and_help_text_bool("Scan", "fs", "Fast seek mode. When using the -fs option with -st do not decode all video before the point we are trying to locate, but instead try to jump directly to it. This will speed up processing but might not find the defined position accurately. Accuracy depends on file format.")
	scan_mode_only := store_options_and_help_text_bool("Scan", "scan", "Scan input files and print video audio and subtitle stream info.")
	split_times := store_options_and_help_text_string("Scan", "sf", "", "Split out parts of the file. Give start and stop times for the parts of the file to use. Use either commas and slashes or only commas to separate time values. Example: -sf 0-10:00,01:35:12.800-01:52:14 defines that 0 secs - 10 mins from the start of the file will be used and joined to the next part that starts from 01 hours 35 mins 12 seconds and 800 milliseconds and ends at 01 hours 52 mins 14 seconds. Don't use space - characters. A zero or word 'start' can be used to mark the absolute start of the file and word 'end' the end of the file. Both start and stop times must be defined. Warning while using options -s -sn -sm and -smn: If your cut point is in the middle of a subtitle presentation time (even when muxing subtitles) you may get a video glitch.")
	search_start_option := store_options_and_help_text_string("Scan", "st", "", "Start time. Start video processing from this timecode. Example -st 30:00 starts processing from 30 minutes from the start of the file.")
	processing_stop_time := store_options_and_help_text_string("Scan", "et", "", "End time. Stop video processing at this timecode. Example -et 01:30:00 stops processing at 1 hour 30 minutes. You can define a time range like this: -st 10:09 -et 01:22:49.500 This results in a video file that starts at 10 minutes 9 seconds and stops at 1 hour 22 minutes, 49 seconds and 500 milliseconds.")
	processing_duration := store_options_and_help_text_string("Scan", "d", "", "Duration of video to process. Example -d 01:02 process 1 minutes and 2 seconds of the file. Use either -et or -d option not both.")

	// Misc options
	// If you want to print debug info then change debug to "true" below
	debug_option := store_options_and_help_text_bool("Misc", "debug", "Turn on debug mode and show info about internal variables and the FFmpeg commandlines used.")
	use_matroska_container := store_options_and_help_text_bool("Misc", "mkv", "Use matroska (mkv) as the output file wrapper format.")
	only_print_commands := store_options_and_help_text_bool("Misc", "print", "Print FFmpeg commands that would be used for processing, don't process any files.")
	show_program_version_short := store_options_and_help_text_bool("Misc", "v", "Show the version of FFcommander.")
	show_program_version_long := store_options_and_help_text_bool("Misc", "version", "Show the version of FFcommander.")
	temp_file_directory := store_options_and_help_text_string("Misc", "td", "", "Path to directory for temporary files, example_ -td PathToDir. This option directs temporary files created with 2-pass encoding and subtitle processing (-sp) to a separate directory. Processing with the -sp switch goes much faster when temporary files are created on a ram or ssd - disk. The -sp switch extracts every frame of a movie as a tiff image, so you need to have lots of free space in the temp directory. For a FullHD movie you need 20 GB or more storage for temporary files. Subtitle extraction with the -sp switch fails silently if you run out of storage space. If this happens then some of the last subtitles won't be available when the video is compressed and this results the last available subtitle being 'stuck' on top of video until the end of the movie. This is a limitation in how FFmpeg works and cannot be worked around.")
	help := store_options_and_help_text_bool("Misc", "h", "Display help text")

	//////////////////////
	// Define variables //
	//////////////////////

	var input_filenames []string
	var deinterlace_options string
	var grayscale_options string
	var subtitle_processing_options string
	var timecode_burn_options string
	var ffmpeg_pass_1_commandline []string
	var ffmpeg_pass_2_commandline []string
	var ffmpeg_subtitle_extract_commandline []string
	var ffmpeg_file_split_commandline []string
	var final_crop_string string
	var command_to_run_str_slice []string
	var file_to_process, video_width, video_height, video_duration, video_codec_name, color_subsampling, color_space string
	var video_height_int int
	var main_video_2_pass_bitrate_str string
	var audio_language, for_visually_impared, number_of_audio_channels, audio_codec string
	var subtitle_language, for_hearing_impared, subtitle_codec_name string
	var crop_values_picture_width int
	var crop_values_picture_height int
	var crop_values_width_offset int
	var crop_values_height_offset int
	var unsorted_ffprobe_information_str_slice []string
	var ffprobe_error_message []string
	var error_code error
	var error_messages_map = make (map[string][]string)
	var file_counter int
	var file_counter_str string
	var files_to_process_str string
	var subtitle_horizontal_offset_int int
	var subtitle_horizontal_offset_str string
	var cut_list_seconds_str_slice []string
	var split_video bool
	var split_info_filename string
	var split_info_file_absolute_path string
	var list_of_splitfiles []string
	var cut_positions_as_timecodes []string
	var timecode_font_size int
	var orig_subtitle_path, cropped_subtitle_path string
	var selected_streams = make(map[string][]string)

	start_time := time.Now()
	file_split_start_time := time.Now()
	file_split_elapsed_time := time.Since(file_split_start_time)
	pass_1_start_time := time.Now()
	pass_1_elapsed_time := time.Since(pass_1_start_time)
	pass_2_start_time := time.Now()
	pass_2_elapsed_time := time.Since(pass_2_start_time)
	subtitle_extract_start_time := time.Now()
	subtitle_extract_elapsed_time := time.Since(subtitle_extract_start_time)
	subtitle_processing_start_time := time.Now()
	subtitle_processing_elapsed_time := time.Since(subtitle_extract_start_time)

	output_directory_name := "00-processed_files"
	sd_directory_name := "sd"
	subtitle_extract_dir := "subtitles"
	original_subtitles_dir := "original_subtitles"
	fixed_subtitles_dir := "fixed_subtitles"

	///////////////////////////////
	// Parse commandline options //
	///////////////////////////////
	input_filenames = parse_options()

	if help.is_turned_on == true {
		display_help_text()
	}
	/////////////////////////////////////////////////////////
	// Test if needed executables can be found in the path //
	/////////////////////////////////////////////////////////
	find_executable_path("ffmpeg")
	find_executable_path("ffprobe")

	if subtitle_burn_split.is_turned_on == true {
		find_executable_path("magick") // Starting from ImageMagick 7 the "magick" command should be used instead of the "convert" - command.
		find_executable_path("mogrify")
		os.Setenv("MAGICK_THREAD_LIMIT", "1") // Disable ImageMagick multithreading it only makes processing slower. This sets an environment variable in the os.
	}

	// Test that user gave a string not a number for options -a and -s
	if _, err := strconv.Atoi(audio_language_option.user_string); err == nil {
		fmt.Println()
		fmt.Println("The option -a requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	if _, err := strconv.Atoi(subtitle_language_option.user_string); err == nil {
		fmt.Println()
		fmt.Println("The option -s requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	if subtitle_burn_resize.user_string != "" && subtitle_burn_split.is_turned_on == false {
		fmt.Println()
		fmt.Println("Subtitle resize can only be used with the -sp option, not alone.")
		fmt.Println()
		os.Exit(0)
	}

	// Test if user gave a valid float on the commandline
	if subtitle_burn_resize.user_string != "" {

		subtitle_resize_float, float_parse_error := strconv.ParseFloat(subtitle_burn_resize.user_string, 64)

		if  float_parse_error != nil || subtitle_resize_float == 0.0 {

			fmt.Println("Error:", subtitle_burn_resize.user_string, "is not a valid number.")
			os.Exit(1)
		}
	}

	// Convert time values used in splitting the inputfile to seconds
	if split_times.user_string != "" {
		split_video = true
		use_matroska_container.is_turned_on = true
		cut_list_seconds_str_slice, cut_positions_as_timecodes = process_split_times(split_times.user_string, debug_option.is_turned_on)
	}

	// Parallel SD processing requires defining inputfile seek for each output file or placing the seek before the inputfile.
	// We choose to place the seek before the inputfile. This results using the fast but sometimes inaccurate FFmpeg seek.
	if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
		fast_search.is_turned_on = true
	}

	// Test if user gave more than one audio compression option
	var audio_compression_slice []string

	if audio_compression_ac3.is_turned_on == true {
		audio_compression_slice = append(audio_compression_slice, "-ac3")
	}

	if audio_compression_aac.is_turned_on == true {
		audio_compression_slice = append(audio_compression_slice, "-aac")
	}

	if audio_compression_opus.is_turned_on == true {
		audio_compression_slice = append(audio_compression_slice, "-opus")
	}

	if audio_compression_flac.is_turned_on == true {
		audio_compression_slice = append(audio_compression_slice, "-flac")
	}

	if len(audio_compression_slice) > 1 {
		fmt.Println()
		fmt.Printf("Error: more than one audio compression options on the commandline: ")

		for _, item := range(audio_compression_slice) {
			fmt.Printf(item + " ")
		}

		fmt.Println("\n")
		os.Exit(0)
	}

	if force_lossless.is_turned_on == true && len(audio_compression_slice) >= 1 {
		fmt.Println()
		fmt.Printf("Error: -ls (lossless processing) cannot be used at the same time as audio compression options: ")

			for _, item := range(audio_compression_slice) {
				fmt.Printf(item + " ")
			}

		fmt.Println("\n")
		os.Exit(0)
	}

	if audio_language_option.is_turned_on == true && audio_stream_number_option.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -a and -an can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if no_audio.is_turned_on == true {

		if audio_language_option.is_turned_on == true || audio_stream_number_option.is_turned_on == true {
			fmt.Println()
			fmt.Printf("Error: -na (no audio) cannot be used at the same time as audio selection options.")
			fmt.Println()
			os.Exit(0)
		}
	}

	if crf_option.is_turned_on == true {

		if fast_encode_and_search.is_turned_on == true || fast_encode.is_turned_on == true {
			fmt.Println()
			fmt.Println("Error: option -crf can't be used at the same time as options -f or -fe.")
			fmt.Println()
			os.Exit(0)
		}

		if main_bitrate_option.is_turned_on == true {
			fmt.Println()
			fmt.Println("Error: options -crf and -mbr can't be used at the same time.")
			fmt.Println()
			os.Exit(0)
		}
	}

	if processing_stop_time.is_turned_on == true && processing_duration.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -et and -d can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if subtitle_language_option.is_turned_on == true && subtitle_stream_number_option.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -s and -sn can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if subtitle_burn_split.is_turned_on == true && subtitle_vertical_offset.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -sp and -so can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if split_times.is_turned_on == true {

		if search_start_option.is_turned_on == true || processing_stop_time.is_turned_on == true {
			fmt.Println()
			fmt.Println("Error: options -sp can't be used at the same time as -st or -et.")
			fmt.Println()
			os.Exit(0)
		}
	}

	// -f option turns on both options -fs and -fe
	if fast_encode_and_search.is_turned_on == true {
		fast_search.is_turned_on = true
		fast_encode.is_turned_on = true
	}

	if search_start_option.user_string == "" && processing_stop_time.user_string != "" {
			fmt.Println()
			fmt.Println("Error, start time missing")
			fmt.Println()
			os.Exit(0)
	}

	// Convert processing end time to duration and store it in variable used with -d option (duration).
	// FFmpeg does not understarnd end times, only start time + duration.
	if processing_stop_time.user_string != "" {

		start_time := ""
		end_time := ""
		error_happened := ""

		if search_start_option.user_string != "" {
			start_time, error_happened = convert_timecode_to_seconds(search_start_option.user_string)

			if error_happened != "" {
				fmt.Println("\nError when converting start time to seconds: " + error_happened + "\n")
				os.Exit(1)
			}
		}

		start_time_int, atoi_error := strconv.Atoi(start_time)

		if atoi_error != nil {
			fmt.Println()
			fmt.Println("Error converting start time", search_start_option.user_string, "to integer")
			fmt.Println()
			os.Exit(0)
		}

		end_time, error_happened = convert_timecode_to_seconds(processing_stop_time.user_string)

		if error_happened != "" {
			fmt.Println("\nError when converting end time to seconds: " + error_happened + "\n")
			os.Exit(1)
		}

		end_time_int, atoi_error := strconv.Atoi(end_time)

		if atoi_error != nil {
			fmt.Println()
			fmt.Println("Error converting end time", processing_stop_time.user_string, "to integer")
			fmt.Println()
			os.Exit(0)
		}

		duration_int := end_time_int - start_time_int

		if duration_int <= 0 {
			fmt.Println()
			fmt.Println("Error duration cannot be", duration_int)
			fmt.Println()
			os.Exit(0)
		}

		processing_duration.user_string = convert_seconds_to_timecode(strconv.Itoa(duration_int))
	}

	// Disable -st and -d options if user did use the -sf option and input some edit times
	if split_video == true {
		search_start_option.user_string = ""
		processing_duration.user_string = ""
	}

	// Always use 1-pass encoding with lossless encoding. Turn on option -fe.
	if force_lossless.is_turned_on == true {
		fast_encode.is_turned_on = true
	}

	if subtitle_burn_split.is_turned_on == true && search_start_option.user_string != "" && fast_encode_and_search.is_turned_on == false && crf_option.is_turned_on == false {
		fmt.Println("\nOptions -st -sp and 2-pass encoding won't work correctly together.")
		fmt.Println("You options are:")
		fmt.Println("disable 2-pass encoding with the -f option")
		fmt.Println("don't use the -st and -et options")
		fmt.Println("use the -crf option (Constant Quality).\n")
		os.Exit(1)
	}

	// Check dvd palette hacking option string correctness.
	if subtitle_burn_palette.user_string != "" {
		temp_slice := strings.Split(subtitle_burn_palette.user_string, ",")
		subtitle_burn_palette.user_string = ""
		hex_characters := [17]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}

		// Test that all characters are valid hex
		for _, character := range temp_slice {

			hex_match_found := false

			if character == "" {
				fmt.Println("")
				fmt.Println("Illegal character: 'empty' in -palette option string. Values must be hex ranging from 0 to f.")
				fmt.Println("")
				os.Exit(0)
			}
			for _, hex_value := range hex_characters {

				if strings.ToLower(character) == hex_value {
					hex_match_found = true
					break
				}
			}

			if hex_match_found == false {
				fmt.Println("")
				fmt.Println("Illegal character:", character, "in -palette option string. Values must be hex ranging from 0 to f.")
				fmt.Println("")
				os.Exit(0)
			}
		}

		// Test that user gave between 1 to 16 characters
		if len(temp_slice) < 1 {
			fmt.Println("")
			fmt.Println("Too few (", len(temp_slice), ") hex characters in -palette option string. Please give 1 to 16 characters.")
			fmt.Println("")
			os.Exit(0)
		}

		if len(temp_slice) > 16 {
			fmt.Println("")
			fmt.Println("Too many (", len(temp_slice), ") hex characters in -palette option string. Please give 1 to 16 characters.")
			fmt.Println("")
			os.Exit(0)
		}

		// Prepare -palette option string for FFmpeg. It requires 16 hex strings where each consists of 6 hex numbers. Of these every 2 numbers control RBG color.
		// The user is limited here to use only shades between black -> gray -> white.
		for counter, character := range temp_slice {

			subtitle_burn_palette.user_string = subtitle_burn_palette.user_string + strings.Repeat(strings.ToLower(character), 6)

			if counter < len(temp_slice)-1 {
				subtitle_burn_palette.user_string = subtitle_burn_palette.user_string + ","
			}

		}

		if len(temp_slice) < 16 {

			subtitle_burn_palette.user_string = subtitle_burn_palette.user_string + ","

			for counter := len(temp_slice); counter < 16; counter++ {
				subtitle_burn_palette.user_string = subtitle_burn_palette.user_string + "ffffff"

				if counter < 15 {
					subtitle_burn_palette.user_string = subtitle_burn_palette.user_string + ","
				}
			}
		}
	}

	// Parse subtitle list
	var user_subtitle_mux_numbers_slice []string
	var user_subtitle_mux_languages_slice []string
	subtitle_mux_bool := false
	subtitle_burn_bool := false
	highest_subtitle_number_int := -1

	if subtitle_mux_numbers_option.user_string != "" && subtitle_mux_language_option.user_string != "" {
		fmt.Println()
		fmt.Println("Error, -sm and -smn cannot be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	// Parse subtitle numbers list
	if subtitle_mux_numbers_option.user_string != "" {

		user_subtitle_mux_numbers_slice = strings.Split(subtitle_mux_numbers_option.user_string, ",")

		if len(user_subtitle_mux_numbers_slice) == 0 {
			fmt.Println()
			fmt.Println("Error parsing subtitle numbers from: ", subtitle_mux_numbers_option.user_string)
			fmt.Println()
			os.Exit(0)
		}

		// Check that user gave only numbers for the option and store highest subtitle number
		for _, number := range user_subtitle_mux_numbers_slice {

			if number_int, atoi_error := strconv.Atoi(number) ; atoi_error != nil {
				fmt.Println()
				fmt.Println("Error parsing subtitle number:", number, "in:", subtitle_mux_numbers_option.user_string)
				fmt.Println()
				os.Exit(0)

			} else if number_int > highest_subtitle_number_int {
				highest_subtitle_number_int = number_int
			}
		}

		subtitle_mux_bool = true
	}

	// Parse subtitle language code list
	if subtitle_mux_language_option.user_string != "" {

		user_subtitle_mux_languages_slice = strings.Split(subtitle_mux_language_option.user_string, ",")

		if len(user_subtitle_mux_languages_slice) == 0 {
			fmt.Println()
			fmt.Println("Error parsing subtitle languages from: ", subtitle_mux_language_option.user_string)
			fmt.Println()
			os.Exit(0)
		}

		subtitle_mux_bool = true
	}

	// Check that user given subtitle numer is a number
	var subtitle_burn_number int
	var atoi_error error
	subtitle_burn_vertical_offset_int := 0

	if subtitle_burn_number, atoi_error = strconv.Atoi(subtitle_stream_number_option.user_string) ; atoi_error != nil {
		fmt.Println()
		fmt.Println("Error parsing subtitle number:", subtitle_stream_number_option.user_string)
		fmt.Println()
		os.Exit(0)
	}

	if subtitle_language_option.user_string != "" || subtitle_burn_number  != -1 {
		subtitle_burn_bool = true
	}

	if subtitle_burn_bool == false && subtitle_burn_grayscale.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error, you need to define what subtitle to burn grayscale on top of video.")
		fmt.Println()
		os.Exit(0)
	}

	if subtitle_mux_bool == true && subtitle_burn_bool == true {
		fmt.Println()
		fmt.Println("Error, you can only burn a subtitle on video or mux subtitles to the file, not both at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	// Use the first subtitle if user wants subtitle split but did not specify subtitle number
	if subtitle_burn_split.is_turned_on == true && subtitle_burn_number == -1 {
		subtitle_burn_number = 0
	}

	if subtitle_burn_vertical_offset_int, atoi_error = strconv.Atoi(subtitle_vertical_offset.user_string) ; atoi_error != nil {
		fmt.Println()
		fmt.Println("Error parsing subtitle offset:", subtitle_vertical_offset.user_string)
		fmt.Println()
		os.Exit(0)
	}

	user_main_bitrate_bool := false
	user_sd_bitrate_bool := false


	if default_video_processing == "crf" {
		crf_option.is_turned_on = true
	}

	// Check the validity of the user given main video bitrate
	if len(main_bitrate_option.user_string) > 0  && crf_option.is_turned_on == false{

		if strings.ToLower( string( main_bitrate_option.user_string )[len(main_bitrate_option.user_string) - 1 : ]) != "k" {
			fmt.Println()
			fmt.Println("The -mbr option takes a bitrate in the form of: 8000k don't forget the 'k' at the end of the value")
			fmt.Println()
			os.Exit(0)
		}

		number_str := string(main_bitrate_option.user_string)[0:len(main_bitrate_option.user_string) - 1]
		number_int,err := strconv.Atoi(number_str)

		if err != nil {
			fmt.Println()
			fmt.Println("Error cannot convert value", main_bitrate_option.user_string, "to a number.")
			fmt.Println()
			os.Exit(0)
		}

		if number_int < 1 || number_int > 1000000 {
			fmt.Println()
			fmt.Println("Error the bitrate must be between 1 and 1 000 000, for example: 8000k")
			fmt.Println()
			os.Exit(0)
		}
		user_main_bitrate_bool = true
	}

	// Check the validity of the user given sd - video bitrate
	if len(sd_bitrate_option.user_string) > 0  && crf_option.is_turned_on == false {

		if strings.ToLower( string( sd_bitrate_option.user_string )[len( sd_bitrate_option.user_string) - 1 : ]) != "k" {
			fmt.Println()
			fmt.Println("The -sbr option takes a bitrate in the form of: 1600k don't forget the 'k' at the end of the value")
			fmt.Println()
			os.Exit(0)
		}

		number_str := string(sd_bitrate_option.user_string)[0:len(sd_bitrate_option.user_string) - 1]
		number_int,err := strconv.Atoi(number_str)

		if err != nil {
			fmt.Println()
			fmt.Println("Error cannot convert value", sd_bitrate_option.user_string, "to a number.")
			fmt.Println()
			os.Exit(0)
		}

		if number_int < 1 || number_int > 1000000 {
			fmt.Println()
			fmt.Println("Error the bitrate must be between 1 and 1 000 000, for example: 1600k")
			fmt.Println()
			os.Exit(0)
		}
		user_sd_bitrate_bool = true
	}

	if parallel_sd.is_turned_on == false && scale_to_sd.is_turned_on == false && user_sd_bitrate_bool == true {
		fmt.Println()
		fmt.Println("Error: the -sbr option can be used only with the -psd or -ssd options.")
		fmt.Println()
		os.Exit(0)
	}

	if scale_to_sd.is_turned_on == true && parallel_sd.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -psd and -ssd can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if debug_option.is_turned_on == true && only_print_commands.is_turned_on == true {
		fmt.Println()
		fmt.Println("Error: options -debug and -print can't be used at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	if debug_option.is_turned_on == true {

		fmt.Println()
		fmt.Println("Subtitle numbers:")
		fmt.Println("-----------------")

		for _, rivi := range user_subtitle_mux_numbers_slice {
			fmt.Println(rivi)
		}

		fmt.Println("Highest_subtitle_number", strconv.Itoa(highest_subtitle_number_int))

		fmt.Println()

		fmt.Println("Subtitle languages")
		fmt.Println("------------------")

		for _, rivi := range user_subtitle_mux_languages_slice {
			fmt.Println(rivi)
		}

		fmt.Println()
	}

	// Check if user given path to temp folder exists and if it is writable
	if temp_file_directory.user_string != "" {

		// Get directory info from filesystem
		dir_stat, err := os.Stat(temp_file_directory.user_string)

		if os.IsNotExist(err) {
			fmt.Println()
			fmt.Println("Path to temp file dir:", temp_file_directory.user_string, "does not exist.")
			fmt.Println()
			os.Exit(0)
		}

		if dir_stat.IsDir() == false {
			fmt.Println()
			fmt.Println("Path to -td option:", temp_file_directory.user_string, "is not a directory.")
			fmt.Println()
			os.Exit(0)
		}

		// Test if temp dir is writable
		testfile_path := filepath.Join(temp_file_directory.user_string, "Testfile.txt")
		err = os.WriteFile(testfile_path, []byte("This is a test file and can be deleted\n"), 0644)

		if err != nil {
			fmt.Println()
			fmt.Println("Tempfile path", temp_file_directory.user_string, "is not writable.")
			fmt.Println()
			os.Exit(0)
		}

		os.Remove(testfile_path)
	}

	// Print program version and license info.
	if show_program_version_short.is_turned_on == true || show_program_version_long.is_turned_on == true {
		fmt.Println()
		fmt.Println("(C) Mikael Hartzell 2018.")
		fmt.Println()
		fmt.Println("FFmpeg version 3 or higher is required to use this program.")
		fmt.Println("Subtitle processing with the -sp option requires ImageMagick.")
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

	var ffmpeg_commandline_start []string
	subtitle_stream_image_format := "tiff" // FFmpeg png extract is 30x slower than tiff, thats why we default to tiff.

	// Determine output file container
	output_video_format := []string{"-f", "mp4"}
	output_mp4_filename_extension := ".mp4"
	output_matroska_filename_extension := ".mkv"
	output_filename_extension := output_mp4_filename_extension
	output_matroska_wrapper_format := "matroska"

	if force_lossless.is_turned_on == true || use_matroska_container.is_turned_on == true {
		// Use matroska as the output file wrapper format
		output_video_format = nil
		output_video_format = append(output_video_format, "-f", output_matroska_wrapper_format)
		output_filename_extension = output_matroska_filename_extension
	}

	if no_deinterlace.is_turned_on == true {
		deinterlace_options = "copy"
	} else {
		// Deinterlacing options used to be: "idet,yadif=0:deint=interlaced"  which tries to detect
		// if a frame is interlaced and deinterlaces only those that are.
		// If there was a cut where there was lots of movement in the picture then some interlace
		// remained in a couple of frames after the cut.
		deinterlace_options = "idet,yadif=0:deint=all"
	}

	number_of_physical_processors, err := get_number_of_physical_processors()

	if number_of_physical_processors < 1 || err != nil {
		number_of_physical_processors = 2

		fmt.Println()
		fmt.Println("ERROR, Could not find out the number of physical processors:", err)
		fmt.Println("Using 2 threads for processing")
		fmt.Println()
	}

	// Don't use more than 8 threads to process a single video because it is claimed that video quality goes down when using more than 10 cores with x264 encoder.
	// https://streaminglearningcenter.com/blogs/ffmpeg-command-threads-how-it-affects-quality-and-performance.html
	// Yllä mainitun sivun screenshot löytyy koodin hakemistosta '00-vanhat' nimellä: 'FFmpeg Threads Command How it Affects Quality and Performance.jpg'
	// It is said that this is caused by the fact that the processing threads can't use the results from other threads to optimize compression.
	// Also processing goes sligthly faster when ffmpeg is using max 8 cores.
	// The user may process files in 4 simultaneously by dividing videos to 4 dirs and processing each simultaneously.

	// Decide automatically how many threads to use.
	// There are claims on the internet that using more than 8 threads
	// in h264 processing will hurt quality, because the threads can not use results from other
	// threads to optimize quality. This is why we default to using a maximum of 8 threads,
	// except when creating a main (HD) and SD video simultaneously
	number_of_threads_to_use_for_video_compression := "auto"

	// User wants to override automatic thread count calculation
	if default_max_threads != "" && default_max_threads != "auto" {

		temp_number,err :=  strconv.Atoi(default_max_threads)

		if err != nil {
			fmt.Println()
			fmt.Println("Error, can't understand the value in variable 'default_max_threads'. Defaulting to automatically determining the amount of threads to use.")
			fmt.Println()

			default_max_threads = ""
		} else {
			if temp_number > 0 {
				number_of_threads_to_use_for_video_compression = default_max_threads
			}
		}
	}

	// Calculate automatically based on core count how many threads to use for video processing
	if default_max_threads == "" {

		if number_of_physical_processors >= 8 {
			number_of_threads_to_use_for_video_compression = "8"
		}

		// For parallel HD and SD compression use max 16 cores
		if parallel_sd.is_turned_on == true {

			if number_of_physical_processors >= 16 {
				number_of_threads_to_use_for_video_compression = "16"
			}
		}
	}

	if debug_option.is_turned_on == true {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-hide_banner", "-threads", number_of_threads_to_use_for_video_compression)
	} else {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-loglevel", "level+error", "-threads", number_of_threads_to_use_for_video_compression)
	}

	if subtitle_burn_split.is_turned_on == true && search_start_option.user_string != "" {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "-fflags", "+genpts")
	}

	///////////////////////////////
	// Scan inputfile properties //
	///////////////////////////////

	for _, inputfile_full_path := range input_filenames {

		// Get video info with: ffprobe -loglevel level+error -show_entries format:stream -print_format flat -i InputFile
		command_to_run_str_slice = nil
		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe", "-loglevel", "level+error", "-show_entries", "format:stream", "-print_format", "flat", "-i")

		if debug_option.is_turned_on == true {
			fmt.Println()
			fmt.Println("command_to_run_str_slice:", command_to_run_str_slice, inputfile_full_path)
		}

		command_to_run_str_slice = append(command_to_run_str_slice, inputfile_full_path)

		unsorted_ffprobe_information_str_slice, ffprobe_error_message, error_code = run_external_command(command_to_run_str_slice)

		if error_code != nil {

			fmt.Println("\n\nFFprobe reported error:", "\n")

			if len(unsorted_ffprobe_information_str_slice) != 0 {
				for _, textline := range unsorted_ffprobe_information_str_slice {
					fmt.Println(textline)
				}
			}

			if len(ffprobe_error_message) != 0 {
				for _, textline := range ffprobe_error_message {
					fmt.Println(textline)
				}
			}

			os.Exit(1)
		}

		// Sort info about video and audio streams in the file to a map. This funtion stores data in global variable: Complete_stream_info_map
		sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice)

		// Get specific video and audio stream information. This function stores data in global variable: Complete_file_info_slice
		get_video_and_audio_stream_information(inputfile_full_path)

	}

	if debug_option.is_turned_on == true {

		fmt.Println()
		fmt.Println("Complete_file_info_slices:")

		for _, temp_slice := range Complete_file_info_slice {
			fmt.Println(temp_slice)
		}
	}

	//////////////////////
	// Scan - only mode //
	//////////////////////

	// Only scan the input files, display their stream properties and exit.
	if scan_mode_only.is_turned_on == true {

		for _, file_info_slice := range Complete_file_info_slice {
			video_slice_temp := file_info_slice[0]
			video_slice := video_slice_temp[0]
			audio_slice := file_info_slice[1]
			subtitle_slice := file_info_slice[2]

			file_to_process = video_slice[0]
			video_width = video_slice[1]
			video_height = video_slice[2]
			video_codec_name = video_slice[4]
			color_subsampling = video_slice[5]
			color_space = video_slice[6]
			frame_rate_str := video_slice[7]
			frame_rate_average_str := video_slice[8]

			fmt.Println()
			subtitle_text := "File name '" + file_to_process + "'"
			text_length := len(subtitle_text)
			fmt.Println(subtitle_text)
			fmt.Println(strings.Repeat("-", text_length))

			if frame_rate_str == "29.970" {
				fmt.Println("\033[7mWarning: Video frame rate is 29.970. You may need to pullup (Inverse Telecine) this video with option -it\033[0m")
			}

			fmt.Printf("Video width: %s, height: %s, codec: %s, color subsampling: %s, color space: %s, fps: %s, average fps: %s\n", video_width, video_height, video_codec_name, color_subsampling, color_space, frame_rate_str, frame_rate_average_str)

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

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Test that all input files have a video stream and that the audio and subtitle streams the user wants do exist //
	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	var subtitles_selected_for_muxing_map = make(map[string][]string)
	var audio_stream_number_int int

	for _, file_info_slice := range Complete_file_info_slice {

		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]
		inputfile_full_path := video_slice[0]
		video_width := video_slice[1]
		video_height := video_slice[2]

		audio_slice := file_info_slice[1]
		audio_stream_found := false

		subtitle_slice := file_info_slice[2]

		if video_width == "0" || video_height == "0" {

			var error_messages []string

			if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
				error_messages = error_messages_map[inputfile_full_path]
			}

			error_messages = append(error_messages, "File does not have a video stream.")
			error_messages_map[inputfile_full_path] = error_messages

			break
		}

		////////////////////////////////////////////////////////////////////////////////////////////////////
		// If user gave us the audio language (fin, eng, ita), find the corresponding audio stream number //
		// If no matching audio is found stop the program.                                                //
		////////////////////////////////////////////////////////////////////////////////////////////////////
		if audio_stream_number_int, atoi_error = strconv.Atoi(audio_stream_number_option.user_string) ; atoi_error != nil {

			// If user did not give us a audio number use 0 as the default
			audio_stream_number_int = 0
		}

		if audio_language_option.user_string != "" {

			for audio_stream_number, audio_info := range audio_slice {
				audio_language = audio_info[0]

				if audio_language_option.user_string == audio_language {
					audio_stream_number_int = audio_stream_number
					number_of_audio_channels = audio_info[2]
					audio_codec = audio_info[4]
					audio_stream_found = true
					break
				}
			}

			if audio_stream_found == false {

				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have audio language: " + audio_language_option.user_string)
				error_messages_map[inputfile_full_path] = error_messages
			}

			if debug_option.is_turned_on == true {
				fmt.Println()
				fmt.Printf("Audio: %s was found in audio stream number: %d\n", audio_language_option.user_string, audio_stream_number_int)
				fmt.Println()
			}

		} else {

			// User did not give audio language code (fin, eng, ita). Find the wanted audio stream by number (starts from 0).
			// Either user defined a audio stream number on the commandline or we use the audio stream number 0.

			if audio_stream_number_int  > len(audio_slice) - 1 {

				// The audio stream number is higher than any stream number in the input file.
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have an audio stream number: " + strconv.Itoa(audio_stream_number_int))
				error_messages_map[inputfile_full_path] = error_messages

			} else {

				// There is a audio stream for the audio stream number we have.
				// Either user defined the number on the commandline or we use the default value of 0 (first audio in source file).
				audio_info := audio_slice[audio_stream_number_int]
				number_of_audio_channels = audio_info[2]
				audio_codec = strings.ToLower(audio_info[4])
				audio_stream_found = true
			}
		}

		if default_audio_processing == "aac" {
			audio_compression_aac.is_turned_on = true
		} else if default_audio_processing == "opus" {
			audio_compression_opus.is_turned_on = true
		}


		if audio_codec == "ac-3" {
			audio_codec = "ac3"
		}

		if audio_compression_aac.is_turned_on == true {
			audio_codec = "aac"
		}

		if audio_compression_opus.is_turned_on == true {
			audio_codec = "opus"
		}

		if audio_compression_ac3.is_turned_on == true {
			audio_codec = "ac3"
		}

		if force_lossless.is_turned_on == true {
			audio_codec = "flac"
		}

		if audio_compression_flac.is_turned_on == true {
			audio_codec = "flac"
		}

		number_of_audio_channels_int, _ := strconv.Atoi(number_of_audio_channels)

		if audio_codec == "ac3" && number_of_audio_channels_int > 6 {

			var error_messages []string

			if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
				error_messages = error_messages_map[inputfile_full_path]
			}

			error_messages = append(error_messages, "Error, file has " + number_of_audio_channels + " audio channels, but AC3 supports max 6 channels")
			error_messages_map[inputfile_full_path] = error_messages
		}

		if audio_codec == "flac" && number_of_audio_channels_int > 8 {

			var error_messages []string

			if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
				error_messages = error_messages_map[inputfile_full_path]
			}

			error_messages = append(error_messages, "Error, file has " + number_of_audio_channels + " audio channels, but FLAC supports max 8 channels")
			error_messages_map[inputfile_full_path] = error_messages
		}

		if audio_codec == "aac" && number_of_audio_channels_int > 48 {

			var error_messages []string

			if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
				error_messages = error_messages_map[inputfile_full_path]
			}

			error_messages = append(error_messages, "Error, file has " + number_of_audio_channels + " audio channels, but AAC supports max 48 channels")
			error_messages_map[inputfile_full_path] = error_messages
		}

		if audio_codec == "opus" && number_of_audio_channels_int > 255 {

			var error_messages []string

			if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
				error_messages = error_messages_map[inputfile_full_path]
			}

			error_messages = append(error_messages, "Error, file has " + number_of_audio_channels + " audio channels, but Opus supports max 255 channels")
			error_messages_map[inputfile_full_path] = error_messages
		}

		// Test if output audio codec is compatible with the mp4 wrapper format
		// MP4 supported audio formats: https://en.wikipedia.org/wiki/Comparison_of_video_container_formats
		// Amr, mp1, mp2, mp3. aac, ac3, e-ac3, dts, opus, alac, mlp, Dolby TrueHD, DTS-HD, als, sls, lpcm, DV Audio.
		if use_matroska_container.is_turned_on == false && audio_stream_found == true {

			if audio_codec != "aac" && audio_codec != "ac3" && audio_codec != "mp2" && audio_codec != "mp3" && audio_codec != "dts" && audio_codec != "opus" {

				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, audio codec: " + audio_codec + " in file is not compatible with the mp4 wrapper format.")
				error_messages = append(error_messages, "Compatible formats are: aac, ac3, mp2, mp3, dts, opus.")
				error_messages = append(error_messages, "Your options are: use the -mkv switch to export to a matroska file, the -aac or -opus switches to convert audio to aac or opus formats.")
				error_messages = append(error_messages, "")
				error_messages_map[inputfile_full_path] = error_messages
			}
		}

		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// If user gave us the subtitle language (fin, eng, ita) to burn on top of video, find the corresponding subtitle stream number //
		// If no matching subtitle is found stop the program.                                                                           //
		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		if subtitle_burn_bool == true && subtitle_language_option.user_string != "" {

			subtitle_found := false
			subtitle_palette_supported := true
			subtitle_burn_supported := true
			subtitle_format := ""

			for counter, subtitle_info := range subtitle_slice {
				// Subtitle found
				subtitle_language = subtitle_info[0]

				if subtitle_language_option.user_string == subtitle_language {
					subtitle_burn_number = counter
					subtitle_found = true
					subtitle_format = subtitle_info[2]

					if subtitle_format != "dvd_subtitle" && subtitle_format != "dvb_subtitle" && subtitle_format != "hdmv_pgs_subtitle" {
						subtitle_burn_supported = false
					}

					if subtitle_burn_palette.user_string != "" {
						if subtitle_format != "dvd_subtitle" || subtitle_format != "dvb_subtitle" {
							subtitle_palette_supported = false
						}
					}

					break // Continue searching the next file when the first matching subtitle has been found.
				}
			}

			if subtitle_found == false {
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have subtitle language: " + subtitle_language_option.user_string)
				error_messages_map[inputfile_full_path] = error_messages
			}

			if subtitle_burn_supported == false {
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, the subtitle format: " + subtitle_format + " is not supported for burning on top of video.\nOnly formats: 'dvd_subtitle', 'dvb_subtitle' and 'hdmv_pgs_subtitle' are supported.")
				error_messages_map[inputfile_full_path] = error_messages
			}

			if subtitle_burn_palette.user_string != "" && subtitle_palette_supported == false {
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, the palette-command does not support subtitle format: " + subtitle_format + ". Only formats: 'dvd_subtitle' and 'dvb_subtitle' are supported.")
				error_messages_map[inputfile_full_path] = error_messages
			}

			if debug_option.is_turned_on == true {
				fmt.Println()
				fmt.Printf("Subtitle: %s was found in file %s as number %s\n", subtitle_language_option.user_string, inputfile_full_path, strconv.Itoa(subtitle_burn_number))
				fmt.Println()
			}

		} else if subtitle_burn_bool == true && subtitle_burn_number != -1 {

			// If user gave subtitle stream number, check that we have at least that much subtitle streams in the source file.
			if subtitle_burn_number > len(subtitle_slice) - 1 {

				// The subtitle number was not found
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have an subtitle stream number: " + strconv.Itoa(subtitle_burn_number))
				error_messages_map[inputfile_full_path] = error_messages

			} else {

				// The subtitle number was found
				temp_slice := subtitle_slice[subtitle_burn_number]
				subtitle_format := temp_slice[2]

				if  subtitle_format != "dvd_subtitle" && subtitle_format != "dvb_subtitle" && subtitle_format != "hdmv_pgs_subtitle" {

					var error_messages []string

					if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
						error_messages = error_messages_map[inputfile_full_path]
					}

					error_messages = append(error_messages, "Error, the subtitle format: " + subtitle_format + " is not supported for burning on top of video.\nOnly formats: 'dvd_subtitle', 'dvb_subtitle' and 'hdmv_pgs_subtitle' are supported.")
					error_messages_map[inputfile_full_path] = error_messages

				}

				if subtitle_burn_palette.user_string != "" {

					if subtitle_format != "dvd_subtitle" || subtitle_format != "dvb_subtitle" {

						var error_messages []string

						if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
							error_messages = error_messages_map[inputfile_full_path]
						}

						error_messages = append(error_messages, "Error, the palette-command does not support subtitle format: " + subtitle_format + ". Only formats: 'dvd_subtitle' and 'dvb_subtitle' are supported.")
						error_messages_map[inputfile_full_path] = error_messages
					}
				}
			}
		}

		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// If user gave us subtitle language codes (fin, eng, ita) to mux into the file, find the corresponding subtitle stream numbers //
		// If no matching subtitle is found stop the program.                                                                           //
		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

		subtitle_found := false
		subtitle_type := ""

		if subtitle_mux_bool == true {

			if len(user_subtitle_mux_languages_slice) > 0 {

				for _, user_sub_language := range user_subtitle_mux_languages_slice {

					for counter, subtitle_info := range subtitle_slice {

						subtitle_found = false
						subtitle_language = subtitle_info[0]
						subtitle_type = subtitle_info[2]

						if user_sub_language == subtitle_language {

							user_subtitle_mux_numbers_slice = append(user_subtitle_mux_numbers_slice, strconv.Itoa(counter))
							subtitle_found = true
							break // Continue searching the next file when the first matching subtitle has been found.
						}
					}

					if subtitle_found == false {

						var error_messages []string

						if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
							error_messages = error_messages_map[inputfile_full_path]
						}

						error_messages = append(error_messages, "Error, file does not have subtitle language: " + user_sub_language)
						error_messages_map[inputfile_full_path] = error_messages
					}

					if debug_option.is_turned_on == true {
						fmt.Println()
						fmt.Printf("Subtitle: %s was found in file %s\n", user_sub_language, inputfile_full_path)
						fmt.Println()
					}

					// Test if output subtitle type is compatible with the mp4 wrapper format
					if use_matroska_container.is_turned_on == false && subtitle_type == "hdmv_pgs_subtitle" {

						var error_messages []string

						if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
							error_messages = error_messages_map[inputfile_full_path]
						}

						error_messages = append(error_messages, "Error, subtitle type " + subtitle_type + " in file is not compatible with the mp4 wrapper format.")
						error_messages = append(error_messages, "Use the -mkv switch to export to a matroska file.")
						error_messages = append(error_messages, "")
						error_messages_map[inputfile_full_path] = error_messages

					}
				}
			}

			// Check that we have at least as much subtitle streams in the source file as the highest subtitle number user gave us.
			if highest_subtitle_number_int > len(subtitle_slice) - 1 {

				// The subtitle number was not found
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have an subtitle stream number: " + strconv.Itoa(highest_subtitle_number_int))
				error_messages_map[inputfile_full_path] = error_messages
			}
		}

		// Store info about selected video  always stream 0), audio and subtitle streams.
		if len(error_messages_map) == 0 {
			var selected_streams_temp []string
			selected_streams_temp = append(selected_streams_temp, "0", strconv.Itoa(audio_stream_number_int), strconv.Itoa(subtitle_burn_number))
			selected_streams[inputfile_full_path] = selected_streams_temp
		}

		// Store selected subtitles to mux to a map
		if subtitle_mux_bool == true && len(user_subtitle_mux_numbers_slice) >0 {
			subtitles_selected_for_muxing_map[inputfile_full_path] = user_subtitle_mux_numbers_slice
			user_subtitle_mux_numbers_slice = nil
		}
	}

	// If there were error messages then we can't process all files that the user gave on the commandline, inform the user and exit.
	if len(error_messages_map) > 0 {

		// Sort file names
		var filenames []string

		for key := range error_messages_map {
			filenames = append(filenames, key)
		}

		sort.Strings(filenames)

		// Print error messages for each file.
		for _, inputfile_full_path := range filenames {

			error_messages := error_messages_map[inputfile_full_path]

			fmt.Println()
			fmt.Println(inputfile_full_path)
			fmt.Println(strings.Repeat("-", len(inputfile_full_path) + 1 ))

			for _, text_line := range error_messages {
				fmt.Println(text_line)
			}
		}

		fmt.Println()
		os.Exit(1)
	}

	/////////////////////////////////////////
	// Main loop that processess all files //
	/////////////////////////////////////////

	if len(Complete_file_info_slice) == 0 {
		fmt.Println()
		fmt.Println("No files to process")
		fmt.Println()
		os.Exit(0)
	}

	files_to_process_str = strconv.Itoa(len(Complete_file_info_slice))

	for _, file_info_slice := range Complete_file_info_slice {

		subtitle_horizontal_offset_int = 0
		subtitle_horizontal_offset_str = "0"
		start_time = time.Now()
		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]
		inputfile_full_path := video_slice[0]
		video_width = video_slice[1]
		video_height = video_slice[2]
		video_duration = video_slice[3]
		video_codec_name = video_slice[4]
		color_subsampling = video_slice[5]
		color_space = video_slice[6]
		frame_rate_str := video_slice[7]

		// Create input + output filenames and paths
		inputfile_path := filepath.Dir(inputfile_full_path)
		inputfile_name := filepath.Base(inputfile_full_path)
		input_filename_extension := filepath.Ext(inputfile_name)
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension) + output_filename_extension)
		subtitle_extract_base_path := filepath.Join(inputfile_path, output_directory_name, subtitle_extract_dir)
		sd_directory_path := filepath.Join(inputfile_path, output_directory_name, sd_directory_name)
		sd_output_file_absolute_path := filepath.Join(sd_directory_path, strings.TrimSuffix(inputfile_name, input_filename_extension) + output_filename_extension)

		if temp_file_directory.user_string != "" {
			subtitle_extract_base_path = filepath.Join(temp_file_directory.user_string, output_directory_name, subtitle_extract_dir)
		}

		original_subtitles_absolute_path := filepath.Join(subtitle_extract_base_path, inputfile_name + "-" + original_subtitles_dir)
		fixed_subtitles_absolute_path := filepath.Join(subtitle_extract_base_path, inputfile_name + "-" + fixed_subtitles_dir)

		if debug_option.is_turned_on == true {
			fmt.Println("inputfile_path:", inputfile_path)
			fmt.Println("inputfile_name:", inputfile_name)
			fmt.Println("output_file_absolute_path:", output_file_absolute_path)
			fmt.Println("video_width:", video_width)
			fmt.Println("video_height:", video_height)
			fmt.Println("orig_subtitle_path", orig_subtitle_path)
			fmt.Println("cropped_subtitle_path", cropped_subtitle_path)
			fmt.Println("subtitle_extract_base_path", subtitle_extract_base_path)
			fmt.Println("original_subtitles_absolute_path", original_subtitles_absolute_path)
			fmt.Println("fixed_subtitles_absolute_path", fixed_subtitles_absolute_path)
			fmt.Println("number_of_physical_processors", number_of_physical_processors)
		}

		// Get selected subtitles to mux from map
		user_subtitle_mux_numbers_slice = subtitles_selected_for_muxing_map[inputfile_full_path]

		// Add messages to processing log.
		var log_messages_str_slice []string
		log_messages_str_slice = append(log_messages_str_slice, "")
		log_messages_str_slice = append(log_messages_str_slice, "Filename: "+inputfile_full_path)
		underline_length := len(inputfile_full_path) + len("Filename: ") + 1
		log_messages_str_slice = append(log_messages_str_slice, strings.Repeat("-", underline_length))
		log_messages_str_slice = append(log_messages_str_slice, "")
		log_messages_str_slice = append(log_messages_str_slice, "Commandline options:")
		log_messages_str_slice = append(log_messages_str_slice, "---------------------")
		log_messages_str_slice = append(log_messages_str_slice, strings.Join(os.Args, " "))

		// If output directory does not exist path then create it.
		if _, err := os.Stat(filepath.Join(inputfile_path, output_directory_name)); os.IsNotExist(err) {
			os.Mkdir(filepath.Join(inputfile_path, output_directory_name), 0777)
		}

		if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
			// If SD output directory does not exist path then create it.
			if _, err := os.Stat(sd_directory_path); os.IsNotExist(err) {
				os.Mkdir(sd_directory_path, 0777)
			}
		}

		// Print information about processing
		file_counter = file_counter + 1
		file_counter_str = strconv.Itoa(file_counter)

		fmt.Println("")
		fmt.Println(strings.Repeat("#", 80))
		fmt.Println("")
		fmt.Println("Processing file " + file_counter_str + "/" + files_to_process_str + "  '" + inputfile_name + "'")

		audio_slice := file_info_slice[1]
		audio_info := audio_slice[audio_stream_number_int]
		number_of_audio_channels = audio_info[2]
		audio_codec = audio_info[4]


		selected_streams_slice := selected_streams[inputfile_full_path]
		audio_stream_number_int, _ = strconv.Atoi(selected_streams_slice[1])
		subtitle_burn_number, _ = strconv.Atoi(selected_streams_slice[2])

		////////////////////////////////////////////////////
		// Split out and use only some parts of the video //
		////////////////////////////////////////////////////
		if split_video == true {

			file_split_start_time = time.Now()
			file_index_counter := 0
			list_of_splitfiles = nil

			// Open split_infofile for appending info about file splits
			split_info_filename = "00-" + strings.TrimSuffix(strings.Replace(inputfile_name, "'", "", -1), input_filename_extension) + "-splitfile_info.txt"
			split_info_file_absolute_path = filepath.Join(inputfile_path, output_directory_name, strings.Replace(split_info_filename, "'", "", -1))

			if _, err := os.Stat(split_info_file_absolute_path); err == nil {
				os.Remove(split_info_file_absolute_path)
			}

			// Create a new split info file
			split_info_file_pointer, err := os.OpenFile(split_info_file_absolute_path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
			defer split_info_file_pointer.Close()

			if err != nil {
				fmt.Println("")
				fmt.Println("Error, could not open split info file:", split_info_filename, "for writing.")
				log.Fatal(err)
				os.Exit(0)
			}

			log_messages_str_slice = append(log_messages_str_slice, "\n")
			log_messages_str_slice = append(log_messages_str_slice, "Creating splitfiles:")
			log_messages_str_slice = append(log_messages_str_slice, "--------------------")

			if subtitle_burn_bool == true || subtitle_mux_bool == true {
				fmt.Println("\033[7mWarning: If your cut point is in the middle of a subtitle (even when muxing subtitles) you may get a video glitch\033[0m")
				log_messages_str_slice = append(log_messages_str_slice,"Warning. If your cut point is in the middle of a subtitle (even when muxing subtitles) you may get a video glitch.\n")
			}

			audio_codec = "flac"

			for counter := 0; counter < len(cut_list_seconds_str_slice); counter = counter + 2 {
				file_index_counter++
				splitfile_name := strings.TrimSuffix(strings.Replace(inputfile_name, "'", "", -1), input_filename_extension) + "-splitfile-" + strconv.Itoa(file_index_counter) + output_matroska_filename_extension
				split_file_path := filepath.Join(inputfile_path, output_directory_name, splitfile_name)
				list_of_splitfiles = append(list_of_splitfiles, split_file_path)

				ffmpeg_file_split_commandline = nil
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, ffmpeg_commandline_start...)
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-i", inputfile_full_path, "-ss", cut_list_seconds_str_slice[counter])

				// There is no timecode if the user wants to process to the end of file. Skip the -t FFmpeg option since FFmpeg processes to the end of file without it.
				if len(cut_list_seconds_str_slice)-1 > counter {
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-t", cut_list_seconds_str_slice[counter+1])
				}

				// Put video and subtitle options on FFmpeg commandline
				if subtitle_burn_bool == true {
					// Subtitle burn
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-vcodec", "utvideo", "-map", "0:v:0", "-scodec", "copy", "-map", "0:s:" + strconv.Itoa(subtitle_burn_number))

				} else if subtitle_mux_bool == true {
					// Subtitle mux
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-vcodec", "utvideo", "-map", "0:v:0", "-scodec", "copy")

					for _, subtitle_mux_number := range user_subtitle_mux_numbers_slice {
						ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-map", "0:s:"+ subtitle_mux_number)
					}

				} else {
					// No subtitle
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-vcodec", "utvideo", "-map", "0:v:0", "-sn")
				}

				// Put audio options on FFmpeg commandline
				var audio_options []string

				if no_audio.is_turned_on == true {
					audio_options = append(audio_options, "-an")
				} else {
					audio_options = append(audio_options, "-acodec", "flac", "-map", "0:a:" + strconv.Itoa(audio_stream_number_int))
				}

				for _, item := range audio_options {
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, item)
				}

				// Put target file path on FFmpeg commandline
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, split_file_path)

				if only_print_commands.is_turned_on == false {
					fmt.Println("Creating splitfile: " + splitfile_name)

				}

				log_messages_str_slice = append(log_messages_str_slice, strings.Join(ffmpeg_file_split_commandline, " "))

				// Write split file names to a text file
				if _, err = split_info_file_pointer.WriteString("file '" + splitfile_name + "'\n"); err != nil {
					fmt.Println("")
					fmt.Println("Error, could not write to split info file:", split_info_filename)
					log.Fatal(err)
					os.Exit(0)
				}

				if debug_option.is_turned_on == true || only_print_commands.is_turned_on == true {
					fmt.Println(strings.Join(ffmpeg_file_split_commandline, " "), "\n")
				}

				var file_split_output_temp []string
				var file_split_error_output_temp []string
				var error_code error

				if only_print_commands.is_turned_on == false {
					file_split_output_temp, file_split_error_output_temp, error_code = run_external_command(ffmpeg_file_split_commandline)
				}

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", "\n")

					if len(file_split_output_temp) != 0 {
						for _, textline := range file_split_output_temp {
							fmt.Println(textline)
						}
					}

					if len(file_split_error_output_temp) != 0 {
						for _, textline := range file_split_error_output_temp {
							fmt.Println(textline)
						}
					}

					os.Exit(1)
				}

			}

			audio_stream_number_int = 0

			if subtitle_burn_bool == true {
				// Audio and subtitle stream numbers will now change to 0 in the splitfiles as all other streams have been left out.
				subtitle_burn_number = 0
			}

			if subtitle_mux_bool == true {
				// Muxed subtitles will now be numbered; 0, 1, 2, 3... in the splitfiles. Store the new order in the slice.
				number_of_subtitles := len(user_subtitle_mux_numbers_slice)
				user_subtitle_mux_numbers_slice = nil

				for counter := 0 ; counter < number_of_subtitles ; counter++ {
					user_subtitle_mux_numbers_slice = append(user_subtitle_mux_numbers_slice, strconv.Itoa(counter))

				}
			}

			file_split_elapsed_time = time.Since(file_split_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Printf("\nSplitfile creation took %s\n", file_split_elapsed_time.Round(time.Millisecond))
				fmt.Println()
			}

			if only_print_commands.is_turned_on == true {

				fmt.Println("Contents of textfile:", split_info_file_absolute_path)

				split_info_file_pointer, err := os.Open(split_info_file_absolute_path)

				if err == nil {
					text_scanner := bufio.NewScanner(split_info_file_pointer)

					for text_scanner.Scan() {
						fmt.Println(text_scanner.Text())
					}
					fmt.Println()
				} else {
					fmt.Println("Could not open texttile:", split_info_file_absolute_path)
					fmt.Println()
				}
				defer split_info_file_pointer.Close()
			}

			log_messages_str_slice = append(log_messages_str_slice, "\nSplitfile creation took "+file_split_elapsed_time.Round(time.Millisecond).String())

			// If user has not defined audio output codec use aac rather than flac
			if audio_compression_flac.is_turned_on != true && audio_codec == "flac" {
				audio_compression_aac.is_turned_on = true
				audio_codec = "aac"
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
		// The detected left border is at C pixels counting from the left of the picture.
		// Take A pixels starting from C to the right and where we end at is the detected right border of the picture.
		// Pixels on the right of this point will be cropped.
		//
		// The detected upper border is at D pixels from the top of the picture
		// Take B pixels starting from D down and where we end at is the detected bottom border of the picture.
		// Pixels below this point will be cropped.
		//

		if autocrop_option.is_turned_on == true {

			// Create the FFmpeg commandline to scan for black areas at the borders of the video.
			command_to_run_str_slice = nil
			quick_scan_failed := false

			// Clear crop value storage map by creating a new map with the same name.
			var crop_value_map = make(map[string]int)
			var crop_start_seconds_int int
			var crop_scan_duration_int int
			var crop_scan_stop_time_int int
			var conversion_error string

			var user_defined_search_start_str string
			var user_defined_search_start_seconds_str string
			var user_defined_search_start_seconds_int int

			var user_defined_video_duration_str string
			var user_defined_video_duration_seconds_str string
			var user_defined_video_duration_seconds_int int

			var atoi_error error

			video_duration_int, _ := strconv.Atoi(strings.Split(video_duration, ".")[0])

			user_defined_search_start_str = search_start_option.user_string

			if user_defined_search_start_str == "" {
				user_defined_search_start_str ="0"
			}

			if strings.Contains(user_defined_search_start_str, ":") || strings.Contains(user_defined_search_start_str, ".") {

				user_defined_search_start_seconds_str, conversion_error = convert_timecode_to_seconds(user_defined_search_start_str)

				if conversion_error !="" {
					fmt.Println("Error converting value:", user_defined_search_start_str, "to seconds.")
					os.Exit(1)
				}

				user_defined_search_start_seconds_int, atoi_error = strconv.Atoi(user_defined_search_start_seconds_str)

				if atoi_error != nil {
					fmt.Println("Error converting value:", user_defined_search_start_seconds_str, "to seconds.")
					os.Exit(1)
				}

				if user_defined_search_start_seconds_int >= video_duration_int {
					fmt.Println("Option -st ", user_defined_search_start_seconds_int, "cannot start ouside video duration", video_duration_int)
					os.Exit(1)
				}
			}

			user_defined_video_duration_str = processing_duration.user_string

			if user_defined_video_duration_str == "" {
				user_defined_video_duration_str = "0"
			}

			if strings.Contains(user_defined_video_duration_str, ":") || strings.Contains(user_defined_video_duration_str, ".") {

				user_defined_video_duration_seconds_str, conversion_error = convert_timecode_to_seconds(user_defined_video_duration_str)

				if conversion_error !="" {
					fmt.Println("Error converting value:", user_defined_video_duration_str, "to seconds.")
					os.Exit(1)
				}

				user_defined_video_duration_seconds_int, atoi_error = strconv.Atoi(user_defined_video_duration_seconds_str)

				if atoi_error != nil {
					fmt.Println("Error converting value:", user_defined_video_duration_seconds_int, "to seconds.")
					os.Exit(1)
				}

				if user_defined_video_duration_seconds_int > video_duration_int {
					fmt.Println("Option -d ", user_defined_video_duration_seconds_int, "cannot be longer than video duration", video_duration_int)
					os.Exit(1)
				}

				if user_defined_search_start_seconds_int + user_defined_video_duration_seconds_int > video_duration_int {
					fmt.Println("Times given with options -d and -st combined:", user_defined_search_start_seconds_int + user_defined_video_duration_seconds_int, "are outside video duration", video_duration_int)
					os.Exit(1)
				}
			}

			crop_start_seconds_int = 0
			crop_scan_duration_int = video_duration_int
			crop_scan_stop_time_int = video_duration_int

			if user_defined_search_start_seconds_int > 0 {
				crop_start_seconds_int = user_defined_search_start_seconds_int
				crop_scan_duration_int = video_duration_int - crop_start_seconds_int
			}

			if user_defined_video_duration_seconds_int > 0 {
				crop_scan_duration_int = user_defined_video_duration_seconds_int
				crop_scan_stop_time_int = user_defined_video_duration_seconds_int + user_defined_search_start_seconds_int
			}

			if debug_option.is_turned_on == true {
				fmt.Println("user_defined_search_start_seconds_str:", user_defined_search_start_seconds_str)
				fmt.Println("user_defined_search_start_seconds_int:", user_defined_search_start_seconds_int)
				fmt.Println("user_defined_video_duration_seconds_str:", user_defined_video_duration_seconds_str)
				fmt.Println("user_defined_video_duration_seconds_int:", user_defined_video_duration_seconds_int)
				fmt.Println("video_duration_int:", video_duration_int)
				fmt.Println("crop_start_seconds_int:", crop_start_seconds_int)
				fmt.Println("crop_scan_duration_int:", crop_scan_duration_int)
				fmt.Println("crop_scan_stop_time_int:", crop_scan_stop_time_int)
				fmt.Println("spotcheck_interval:", crop_scan_duration_int / 10)
			}

			// For long videos take short snapshots of crop values spanning the whole file. This is "quick scan mode".
			if crop_scan_duration_int > 300 {

				spotcheck_interval := crop_scan_duration_int / 10 // How many spot checks will be made across the duration of the video (default = 10)
				scan_duration_str := "10"                     // How many seconds of video to scan for each spot (default = 10 seconds)
				scan_duration_int, _ := strconv.Atoi(scan_duration_str)

				if debug_option.is_turned_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				// Repeat spot checks
				for time_to_jump_to := crop_start_seconds_int + scan_duration_int ; time_to_jump_to + scan_duration_int < crop_scan_stop_time_int ; time_to_jump_to = time_to_jump_to + spotcheck_interval {

					// Create the ffmpeg command to scan for crop values
					command_to_run_str_slice = nil
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-ss", strconv.Itoa(time_to_jump_to), "-t", scan_duration_str, "-i", inputfile_full_path, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

					if debug_option.is_turned_on == true {
						fmt.Println()
						fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
						fmt.Println()
					}

					ffmpeg_crop_output, ffmpeg_crop_error_output, error_code := run_external_command(command_to_run_str_slice)

					if error_code != nil {

						fmt.Println("\n\nFFmpeg reported error:", "\n")

						if len(ffmpeg_crop_output) != 0 {
							for _, textline := range ffmpeg_crop_output {
								fmt.Println(textline)
							}
						}

						if len(ffmpeg_crop_error_output) != 0 {
							for _, textline := range ffmpeg_crop_error_output {
								fmt.Println(textline)
							}
						}

						os.Exit(1)
					}

					// Parse the crop value list to find the value that is most frequent, that is the value that can be applied without cropping too much or too little.
					if error_code == nil {

						crop_value_counter := 0

						for _, slice_item := range ffmpeg_crop_error_output {

							for _, item := range strings.Split(slice_item, "\n") {

								if strings.Contains(item, "crop=") {

									crop_value := strings.Split(item, "crop=")[1]

									if _, item_found := crop_value_map[crop_value]; item_found == true {
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
			if crop_scan_duration_int < 300 || quick_scan_failed == true || len(crop_value_map) == 0 {

				if quick_scan_failed == true {
					crop_scan_duration_int = 1800
				}

				command_to_run_str_slice = nil

				if crop_start_seconds_int == 0 {
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-t", strconv.Itoa(crop_scan_duration_int), "-i", inputfile_full_path, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")
				} else {
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-ss", strconv.Itoa(crop_start_seconds_int), "-t", strconv.Itoa(crop_scan_duration_int), "-i", inputfile_full_path, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")
				}

				if debug_option.is_turned_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				if debug_option.is_turned_on == true {
					fmt.Println()
					fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
					fmt.Println()
				}

				ffmpeg_crop_output, ffmpeg_crop_error_output, error_code := run_external_command(command_to_run_str_slice)

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", "\n")

					if len(ffmpeg_crop_output) != 0 {
						for _, textline := range ffmpeg_crop_output {
							fmt.Println(textline)
						}
					}

					if len(ffmpeg_crop_error_output) != 0 {
						for _, textline := range ffmpeg_crop_error_output {
							fmt.Println(textline)
						}
					}

					os.Exit(1)
				}

				// FFmpeg collects possible crop values across the first 1800 seconds of the file and outputs a list of how many times each possible crop values exists.
				// Parse the list to find the value that is most frequent, that is the value that can be applied without cropping too much or too little.
				if error_code == nil {

					crop_value_counter := 0

					for _, slice_item := range ffmpeg_crop_error_output {

						for _, item := range strings.Split(slice_item, "\n") {

							if strings.Contains(item, "crop=") {

								crop_value := strings.Split(item, "crop=")[1]

								if _, item_found := crop_value_map[crop_value]; item_found == true {
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

					if len(ffmpeg_crop_output) != 0 {
						for _, textline := range ffmpeg_crop_output {
							fmt.Println(textline)
						}
					}

					if len(ffmpeg_crop_error_output) != 0 {
						for _, textline := range ffmpeg_crop_error_output {
							fmt.Println(textline)
						}
					}

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
			crop_values_picture_width, _ = strconv.Atoi(strings.Split(final_crop_string, ":")[0])
			crop_values_picture_height, _ = strconv.Atoi(strings.Split(final_crop_string, ":")[1])
			crop_values_width_offset, _ = strconv.Atoi(strings.Split(final_crop_string, ":")[2])
			crop_values_height_offset, _ = strconv.Atoi(strings.Split(final_crop_string, ":")[3])

			/////////////////////////////////////////
			// Print variable values in debug mode //
			/////////////////////////////////////////
			if debug_option.is_turned_on == true {

				fmt.Println()
				fmt.Println("Crop values are:")

				for crop_value := range crop_value_map {
					fmt.Println(crop_value_map[crop_value], "instances of crop values:", crop_value)

				}

				fmt.Println()
				fmt.Println("Most frequent crop value is", final_crop_string)
			}

			video_height_int, _ := strconv.Atoi(video_height)
			cropped_height := video_height_int - crop_values_picture_height - crop_values_height_offset

			video_width_int, _ := strconv.Atoi(video_width)
			cropped_width := video_width_int - crop_values_picture_width - crop_values_width_offset

			// Prepare offset for possible subtitle burn in
			// Subtitle placement is always relative to the left side of the picture,
			// if left is cropped then the subtitle needs to be moved left the same amount of pixels
			// Don't use subtitle offset if option -sp is active because it will center subtitles automatically.
			if subtitle_burn_split.is_turned_on == false {
				subtitle_horizontal_offset_int = crop_values_width_offset * -1
				subtitle_horizontal_offset_str = strconv.Itoa(subtitle_horizontal_offset_int)
			}

			fmt.Println("Top:", crop_values_height_offset, ", Bottom:", strconv.Itoa(cropped_height), ", Left:", crop_values_width_offset, ", Right:", strconv.Itoa(cropped_width))

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "Crop values are, Top: "+strconv.Itoa(crop_values_height_offset)+", Bottom: "+strconv.Itoa(cropped_height)+", Left: "+strconv.Itoa(crop_values_width_offset)+", Right: "+strconv.Itoa(cropped_width))
			log_messages_str_slice = append(log_messages_str_slice, "After cropping video width is: "+strconv.Itoa(crop_values_picture_width)+", and height is: "+strconv.Itoa(crop_values_picture_height))
		}

		////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Subtitle Split. Move subtitles that are above the center of the screen up to the top of the screen and subtitles below center down on the bottom of the screen //
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

		if subtitle_burn_split.is_turned_on == true && subtitle_burn_number > -1 {

			var subtitle_extract_output []string
			var subtitle_extract_error_output []string

			if only_print_commands.is_turned_on == false {
				// Remove subtitle directories if they were left over from the previous run
				if _, err := os.Stat(original_subtitles_absolute_path); err == nil {
					fmt.Printf("Deleting original subtitle files left over from previous run. ")

					os.RemoveAll(original_subtitles_absolute_path)

					fmt.Println("Done.")
				}

				if _, err := os.Stat(fixed_subtitles_absolute_path); err == nil {
					fmt.Printf("Deleting fixed subtitle files left over from previous run. ")

					os.RemoveAll(fixed_subtitles_absolute_path)

					fmt.Println("Done.")
				}
			}

			subtitle_extract_start_time = time.Now()

			// Create output subdirectories
			if _, err := os.Stat(original_subtitles_absolute_path); os.IsNotExist(err) {
				os.MkdirAll(original_subtitles_absolute_path, 0777)
			}

			if _, err := os.Stat(fixed_subtitles_absolute_path); os.IsNotExist(err) {
				os.MkdirAll(fixed_subtitles_absolute_path, 0777)
			}

			/////////////////////////////////////////////////////////////////////////////
			// Extract subtitle stream as separate images for every frame of the movie //
			/////////////////////////////////////////////////////////////////////////////
			subtitle_processing_start_time = time.Now()
			ffmpeg_subtitle_extract_commandline = nil
			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, ffmpeg_commandline_start...)


			// If the user wants to use the fast and inaccurate search, place the -ss option before the first -i on ffmpeg commandline.
			if search_start_option.user_string != "" {
				if fast_search.is_turned_on == true || crf_option.is_turned_on == true {
					ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-ss", search_start_option.user_string)
				}
			}

			if split_video == true {

				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-f", "concat", "-safe", "0", "-i", split_info_file_absolute_path)

			} else {
				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-i", inputfile_full_path)
			}

			// The user wants to use the slow and accurate search, place the -ss option after the first -i on ffmpeg commandline.
			if search_start_option.user_string != "" {
				if fast_search.is_turned_on == false && crf_option.is_turned_on == false {
					ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-ss", search_start_option.user_string)
				}
			}

			if processing_duration.user_string != "" {
				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-t", processing_duration.user_string)
			}

			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-vn", "-an", "-filter_complex", "[0:s:" + strconv.Itoa(subtitle_burn_number)+"]copy[subtitle_processing_stream]", "-map", "[subtitle_processing_stream]", filepath.Join(original_subtitles_absolute_path, "subtitle-%10d." + subtitle_stream_image_format))

			if debug_option.is_turned_on == true || only_print_commands.is_turned_on == true {
				fmt.Println()
				fmt.Println("FFmpeg Subtitle Extract Commandline:")
				fmt.Println(strings.Join(ffmpeg_subtitle_extract_commandline, " "))
				fmt.Println()
			}

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Subtitle Extract Options:")
			log_messages_str_slice = append(log_messages_str_slice, "--------------------------------")
			log_messages_str_slice = append(log_messages_str_slice, strings.Join(ffmpeg_subtitle_extract_commandline, " "))

			if only_print_commands.is_turned_on == false {
				fmt.Printf("Extracting subtitle stream as %s - images ", subtitle_stream_image_format)
			}

			error_code = nil

			////////////////
			// Run FFmpeg //
			////////////////
			if only_print_commands.is_turned_on == false {
				subtitle_extract_output, subtitle_extract_error_output, error_code = run_external_command(ffmpeg_subtitle_extract_commandline)
			}

			if error_code != nil {

				fmt.Println("\n\nFFmpeg reported error:", "\n")

				if len(subtitle_extract_output) != 0 {
					for _, textline := range subtitle_extract_output {
						fmt.Println(textline)
					}
				}

				if len(subtitle_extract_error_output) != 0 {
					for _, textline := range subtitle_extract_error_output {
						fmt.Println(textline)
					}
				}

				os.Exit(1)
			}

			if len(subtitle_extract_output) != 0 && strings.TrimSpace(subtitle_extract_output[0]) != "" {
				fmt.Println("\n", subtitle_extract_output, "\n")
			}

			subtitle_extract_elapsed_time = time.Since(subtitle_extract_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Println("took", subtitle_extract_elapsed_time.Round(time.Millisecond))
			}

			//////////////////////////////////////////////////////////////////////////////////////////
			// Process extracted subtitles in as many threads as there are physical processor cores //
			//////////////////////////////////////////////////////////////////////////////////////////

			// Read in subtitle file names
			files_str_slice := read_filenames_in_a_dir(original_subtitles_absolute_path)

			duplicate_removal_start_time := time.Now()
			if only_print_commands.is_turned_on == false {
				fmt.Printf("Removing duplicate subtitle slides ")
			}

			v_height := video_height
			v_width := video_width

			if autocrop_option.is_turned_on == true {
				v_height = strconv.Itoa(crop_values_picture_height)
				v_width = strconv.Itoa(crop_values_picture_width)
			}

			var files_remaining []string

			if only_print_commands.is_turned_on == false {
				files_remaining = remove_duplicate_subtitle_images (original_subtitles_absolute_path, fixed_subtitles_absolute_path, files_str_slice, v_width, v_height)
			}

			duplicate_removal_elapsed_time := time.Since(duplicate_removal_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Println("took", duplicate_removal_elapsed_time.Round(time.Millisecond))
			}

			subtitle_trimming_start_time := time.Now()

			if only_print_commands.is_turned_on == false {

				if subtitle_burn_resize.user_string != "" {

					fmt.Printf("Trimming and resizing subtitle images in " + strconv.Itoa(number_of_physical_processors) + " threads ")

				} else {

					fmt.Printf("Trimming subtitle images in multiple threads ")
				}

				if debug_option.is_turned_on == true {
					fmt.Println()
				}
			}

			number_of_subtitle_files := len(files_remaining)
			subtitles_per_processor := number_of_subtitle_files / number_of_physical_processors

			if subtitles_per_processor < 2 {
				subtitles_per_processor = 2
			}

			subtitle_end_number := 0

			// Start goroutines
			return_channel := make(chan int, number_of_physical_processors + 1)
			process_number := 1

			for subtitle_start_number := 0 ; subtitle_end_number < number_of_subtitle_files ; {

				subtitle_end_number = subtitle_start_number + subtitles_per_processor

				if subtitle_end_number + 1 > number_of_subtitle_files {
					subtitle_end_number = number_of_subtitle_files
				}

				go subtitle_trim(original_subtitles_absolute_path, fixed_subtitles_absolute_path, files_remaining[subtitle_start_number : subtitle_end_number], v_width, v_height, process_number, return_channel, subtitle_burn_resize.user_string, subtitle_burn_grayscale.is_turned_on)

				if debug_option.is_turned_on == true {
					fmt.Println("Process number:", process_number, "started. It processes subtitles:", subtitle_start_number + 1, "-", subtitle_end_number)
				}

				process_number++
				subtitle_start_number =  subtitle_end_number
			}

			// Wait for subtitle processing in goroutines to end
			processes_stopped := 1

			if debug_option.is_turned_on == true {
				fmt.Println()
			}

			for processes_stopped < process_number {
				return_message := <- return_channel

				if debug_option.is_turned_on == true {
					fmt.Println("Process number:", return_message, "ended.")
				}

				processes_stopped++
			}

			subtitle_trimming_elapsed_time := time.Since(subtitle_trimming_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Println("took", subtitle_trimming_elapsed_time.Round(time.Millisecond))
			}

			subtitle_processing_elapsed_time = time.Since(subtitle_processing_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Printf("Complete subtitle processing took %s", subtitle_processing_elapsed_time.Round(time.Millisecond))
				fmt.Println()
			}


			if debug_option.is_turned_on == false && only_print_commands.is_turned_on == false {

				if _, err := os.Stat(original_subtitles_absolute_path); err == nil {
					fmt.Printf("Deleting original subtitles to recover disk space. ")

					os.RemoveAll(original_subtitles_absolute_path)

					fmt.Println("Done.")
				}
			}
		}

		/////////////////////////
		// Encode video - mode //
		/////////////////////////

		/////////////////////////////////////////////////
		// Create the first part of FFmpeg commandline //
		/////////////////////////////////////////////////

		if scan_mode_only.is_turned_on == false {

			ffmpeg_pass_1_commandline = nil
			ffmpeg_pass_2_commandline = nil

			// Set timecode burn font size
			video_height_int, _ = strconv.Atoi(video_height)
			timecode_font_size = 24

			if video_height_int > 699 {
				timecode_font_size = 48
			}

			// Create the start of ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_commandline_start...)

			// If the user wants to use the fast and inaccurate search, place the -ss option before the first -i on ffmpeg commandline.
			if search_start_option.user_string != "" {
				if fast_search.is_turned_on == true || crf_option.is_turned_on == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", search_start_option.user_string)
				}
			}

			// Add possible dvd subtitle color palette hacking option to the FFmpeg commandline.
			// It must be before the first input file to take effect for that file.
			if subtitle_burn_palette.user_string != "" && subtitle_mux_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-palette", subtitle_burn_palette.user_string)
			}

			if split_video == true {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-f", "concat", "-safe", "0", "-i", split_info_file_absolute_path)

				 if subtitle_burn_split.is_turned_on == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-thread_queue_size", "4096", "-f", "image2", "-i", filepath.Join(fixed_subtitles_absolute_path, "subtitle-%10d." + subtitle_stream_image_format))
				}

			} else if subtitle_burn_split.is_turned_on == true {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", inputfile_full_path, "-thread_queue_size", "4096", "-f", "image2", "-i", filepath.Join(fixed_subtitles_absolute_path, "subtitle-%10d." + subtitle_stream_image_format))

			} else {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", inputfile_full_path)
			}

			// The user wants to use the slow and accurate search, place the -ss option after the first -i on ffmpeg commandline.
			if search_start_option.user_string != "" {
				if fast_search.is_turned_on == false && crf_option.is_turned_on == false {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", search_start_option.user_string)
				}
			}

			if parallel_sd.is_turned_on == false && scale_to_sd.is_turned_on == false {
				if processing_duration.user_string != "" {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-t", processing_duration.user_string)
				}
			}

			ffmpeg_filter_options := ""
			ffmpeg_filter_options_2 := ""

			// Create grayscale FFmpeg - options
			if grayscale_option.is_turned_on == false {

				grayscale_options = ""

			} else {

				grayscale_options = "lut=u=128:v=128"
			}

			// Timecode burn options
			if burn_timecode.is_turned_on == true {
				timecode_burn_options = "drawtext=/usr/share/fonts/TTF/LiberationMono-Regular.ttf:text=%{pts \\\\: hms}:fontcolor=#ffc400:fontsize=" +
					strconv.Itoa(timecode_font_size) + ":box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2:y=(text_h/2)"
			}

			/////////////////////////////////////////////////////
			// Create -filter_complex processing chain options //
			/////////////////////////////////////////////////////

			// Add pullup option on the ffmpeg commandline
			if inverse_telecine.is_turned_on == true {
				ffmpeg_filter_options = ffmpeg_filter_options + "pullup"
			}

			// Add deinterlace commands to ffmpeg commandline
			if ffmpeg_filter_options != "" {
				ffmpeg_filter_options = ffmpeg_filter_options + ","
			}
			ffmpeg_filter_options = ffmpeg_filter_options + deinterlace_options

			// Add crop commands to ffmpeg commandline
			if autocrop_option.is_turned_on == true {
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + "crop=" + final_crop_string
			}

			// Add denoise options to ffmpeg commandline
			if denoise_option.is_turned_on == true {
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + strings.Join(denoise_options, "")
			}

			// Add timecode burn in options
			if burn_timecode.is_turned_on == true {
				ffmpeg_filter_options_2 = ffmpeg_filter_options_2 + "," + timecode_burn_options
			}

			// Add grayscale options to ffmpeg commandline
			if grayscale_option.is_turned_on == true {
				ffmpeg_filter_options_2 = ffmpeg_filter_options_2 + "," + grayscale_options
			}

			///////////////////////////////////////////////////////////////////////////////////
			// Calculate resolution of the SD - version created parallel to the HD - version //
			///////////////////////////////////////////////////////////////////////////////////

			sd_width := 0
			v_width := 0
			v_height := 0

			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				// Use cropped video resolution if it is defined else use original wideo reso
				if autocrop_option.is_turned_on == true {

					v_width = crop_values_picture_width
				} else {
					v_width,_ = strconv.Atoi(video_width)
				}

				if crop_values_picture_height > 0 {
					v_height = crop_values_picture_height
				} else {
					v_height,_ = strconv.Atoi(video_height)
				}

				// First try matching the standard HD - resolutions to SD, We only calculate SD x - resolution FFmpeg will automatically scale y based on x.
				if v_width == 1920 {
					sd_width = 1024
				} else if v_height == 1080 {
					sd_width = 720
				} else {

					// The HD resolutions were not the standard ones, calculate aspect radio and decide SD resolution based on it
					// 4:3 = 125 and 16:9 = 177 and wide screen > 177.
					aspect_ratio := (v_width * 100) / v_height

					if aspect_ratio <= 151 {

						sd_width = 720

					} else {
						sd_width = 1024
					}
				}

				if sd_width == 0 {
					fmt.Println()
					fmt.Println("Error: could not calculate width of the SD - version")
					fmt.Println()
					os.Exit(1)
				}
			}

			/////////////////////////////////////////
			// No subtitle in any format is wanted //
			/////////////////////////////////////////
			if subtitle_mux_bool == false && subtitle_burn_number == -1 {

				// There is no subtitle to process add the "no subtitle" option to FFmpeg commandline.
				if parallel_sd.is_turned_on == true {

					// Create a main (HD) and SD - video simultaneously
					// FFmpeg scaling needs only resolution of one axis and it calculates the other automatically. For example for a 1920x1080 source video: scale=1024:-2 will scale the video to 1024x576. The -2 means calculate axis automatically so that it is divisible by 2
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + ",split=2[main_processed_video_out][sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]", "-map", "[main_processed_video_out]", "-sn")

				} else if scale_to_sd.is_turned_on == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + "[sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]")
				} else {

					// Create only one video version
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + "[main_processed_video_out]", "-map", "[main_processed_video_out]", "-sn")
				}
			}

			////////////////////////////////
			// User wants to mux subtitle //
			////////////////////////////////
			if subtitle_mux_bool == true {

				// There is a dvd, dvb or bluray bitmap subtitle to mux into the target file add the relevant options to FFmpeg commandline.
				if parallel_sd.is_turned_on == true {

					// Create a main (HD) and SD - video simultaneously
					// FFmpeg scaling needs only resolution of one axis and it calculates the other automatically. For example for a 1920x1080 source video: scale=1024:-2 will scale the video to 1024x576. The -2 means calculate axis automatically so that it is divisible by 2
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + ",split=2[main_processed_video_out][sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]", "-map", "[main_processed_video_out]")

				} else if scale_to_sd.is_turned_on == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + "[sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]")
				} else {

					// Create only one video version
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:v:0]" + ffmpeg_filter_options + ffmpeg_filter_options_2 + "[main_processed_video_out]", "-map", "[main_processed_video_out]")
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-scodec", "copy")

				for _, subtitle_mux_number := range user_subtitle_mux_numbers_slice {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:s:"+ subtitle_mux_number)
				}

			}

			///////////////////
			// Subtitle burn //
			///////////////////
			if subtitle_burn_number >= 0 {

				// Add video filter options to ffmpeg commanline
				subtitle_processing_options = "copy"

				// When cropping video widthwise shrink subtitles to fit on top of the cropped video.
				// This results in smaller subtitle font.
				if autocrop_option.is_turned_on == true && subtitle_burn_downscale.is_turned_on == true {
					subtitle_processing_options = "scale=" + strconv.Itoa(crop_values_picture_width) + ":" + strconv.Itoa(crop_values_picture_height)
				}

				subtitle_source_file := "[0:s:"

				if subtitle_burn_split.is_turned_on == true {

					subtitle_source_file = "[1:v:"
					subtitle_burn_number = 0
				}

				if parallel_sd.is_turned_on == true {

					// Create a main (HD) and SD - video simultaneously
					// FFmpeg scaling needs only resolution of one axis and it calculates the other automatically. For example for a 1920x1080 source video: scale=1024:-2 will scale the video to 1024x576. The -2 means calculate axis automatically so that it is divisible by 2
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", subtitle_source_file + strconv.Itoa(subtitle_burn_number) +
						"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
						"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
						strconv.Itoa(subtitle_burn_vertical_offset_int) + ffmpeg_filter_options_2 +
						",split=2[main_processed_video_out][sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]", "-map", "[main_processed_video_out]")

				} else if scale_to_sd.is_turned_on == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", subtitle_source_file + strconv.Itoa(subtitle_burn_number) +
						"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
						"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
						strconv.Itoa(subtitle_burn_vertical_offset_int) + ffmpeg_filter_options_2 +
						"[sd_input],[sd_input]scale=" + strconv.Itoa(sd_width) + ":-2[sd_scaled_out]")
				} else {

					// Create only one video version
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", subtitle_source_file + strconv.Itoa(subtitle_burn_number) +
						"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
						"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
						strconv.Itoa(subtitle_burn_vertical_offset_int) + ffmpeg_filter_options_2 +
						"[main_processed_video_out]", "-map", "[main_processed_video_out]")
					}
				}

			// Inverse telecine returns frame rate back to original 24 fps
			if inverse_telecine.is_turned_on == true {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-r", "24")
			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compression options to FFmpeg commandline //
			///////////////////////////////////////////////////////////////////

			sd_ffmpeg_pass_1_commandline := []string{}
			sd_ffmpeg_pass_2_commandline := []string{"-map", "[sd_scaled_out]", "-sws_flags", "lanczos"}

			////////////////////////////////////////////////////////////////////////
			// Calculate 2-pass bitrate based on the pixecount on the video frame //
			////////////////////////////////////////////////////////////////////////

			if autocrop_option.is_turned_on == true {

				v_width = crop_values_picture_width
				v_height = crop_values_picture_height
			} else {
				v_width,_ = strconv.Atoi(video_width)
				v_height,_ = strconv.Atoi(video_height)
			}

			// The formula is (horizontal resolution * vertical resolution) / video_compression_bitrate_divider. For example: 1920 x 1080 = 2 073 600 pixels / 256 = bitrate 8100k
			main_video_2_pass_bitrate_int := (v_width * v_height) / video_compression_bitrate_divider
			main_video_2_pass_bitrate_str = strconv.Itoa(main_video_2_pass_bitrate_int) + "k"

			// User overrides automatic bitrate calculation and defines one on the commandline
			if user_main_bitrate_bool == true {
				main_video_2_pass_bitrate_str = main_bitrate_option.user_string
			}

			// Parallel SD video 2-pass bitrate is fixed to 1620k
			sd_video_bitrate := "1620k"

			// User overrides automatic bitrate calculation and defines one on the commandline
			if user_sd_bitrate_bool == true {
				sd_video_bitrate = sd_bitrate_option.user_string
			}

			/////////////////////////////////////////////////////////////////
			// Choose video compression profile by the vertical resolution //
			/////////////////////////////////////////////////////////////////
			main_video_compression_options := video_compression_options_sd

			if v_height > 4191 {
				main_video_compression_options = video_compression_options_ultra_hd_8k

			} else if v_height > 2096 {
				main_video_compression_options = video_compression_options_ultra_hd_4k

			} else if v_height > 699 {
				main_video_compression_options = video_compression_options_hd
			}

			if crf_option.is_turned_on == true {
				// Use constant quality instead of 2-pass encoding
				main_video_compression_options = append(main_video_compression_options, "-crf", crf_value)
				main_video_2_pass_bitrate_str = "Constant Quality: " + crf_value
			} else {
				// Add calculated 2-pass bitrate to video compression options
				main_video_compression_options = append(main_video_compression_options, "-b:v", main_video_2_pass_bitrate_str)
			}

			if force_lossless.is_turned_on == true {
				// Lossless audio compression options
				audio_compression_options = nil
				audio_compression_options = audio_compression_options_lossless

				// Lossless video compression options
				main_video_compression_options = video_compression_options_lossless
				main_video_2_pass_bitrate_str = "Lossless"
			}

			if audio_compression_flac.is_turned_on == true {
				audio_compression_options = nil
				audio_compression_options = audio_compression_options_lossless
			}

			//////////////////////////
			// Choose audio options //
			//////////////////////////
			number_of_audio_channels_int, _ := strconv.Atoi(number_of_audio_channels)
			bitrate_int := number_of_audio_channels_int * audio_bitrate_multiplier
			bitrate_str := strconv.Itoa(bitrate_int) + "k"

			if audio_compression_aac.is_turned_on == true {

				audio_compression_options = nil
				audio_compression_options = []string{"-c:a", "aac", "-b:a", bitrate_str}

			}

			// FIXME When FFmpeg opus support in mp4 is mainlined, remove "-strict", "-2" options from the couple of lines below
			// If we are encoding audio to opus, then enable FFmpeg experimental features
			// -strict -2 is needed for FFmpeg to use still experimental support for opus in mp4 container.
			// 2020.11.14: FFmpeg 4.3.1 seems to support opus in mp4 withous strict 2, these can be removed from the following lines
			if audio_compression_opus.is_turned_on == true {

				if number_of_audio_channels_int <= 2 {
					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "libopus", "-b:a", bitrate_str, "-vbr", "off", "-mapping_family", "0",  "-strict", "-2"}
				} else {
					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "libopus", "-b:a", bitrate_str, "-vbr", "off", "-mapping_family", "255", "-strict", "-2"}
				}
			}

			// If we are copying opus audio, then enable FFmpeg experimental features
			// -strict -2 is needed for FFmpeg to use still experimental support for opus in mp4 container.
			if audio_codec == "opus" && audio_compression_options[1] == "copy" {
				audio_compression_options = append(audio_compression_options, "-strict", "-2")
			}

			if audio_compression_ac3.is_turned_on == true {

				if bitrate_int > 640 {

					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "ac3", "-b:a", "640k"}
				} else {

					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "ac3", "-b:a", bitrate_str}
				}

			}

			if no_audio.is_turned_on == true {
				audio_compression_options = nil
				audio_compression_options = append(audio_compression_options, "-an")
			}

			// Add subtitle options for parallel SD processing
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				if subtitle_mux_bool == false && subtitle_burn_number == -1 {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-sn")
				}

				if subtitle_mux_bool == true {
					// There is a dvd, dvb or bluray bitmap subtitle to mux into the target file add the relevant options to FFmpeg commandline.
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-scodec", "copy")

					for _, subtitle_mux_number := range user_subtitle_mux_numbers_slice {
						sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-map", "0:s:"+ subtitle_mux_number)
					}
				}
			}

			// Add video compression options to ffmpeg commandline
			if scale_to_sd.is_turned_on == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, main_video_compression_options...)
			}

			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, video_compression_options_sd...)

				if crf_option.is_turned_on == true {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-crf", crf_value)
				} else {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-b:v", sd_video_bitrate)
				}
			}

			if scale_to_sd.is_turned_on == false {
				// Add color subsampling options if needed
				if color_subsampling != "yuv420p" {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, color_subsampling_options...)
				}

				// Add audio compression options to ffmpeg commandline
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, audio_compression_options...)

				if no_audio.is_turned_on == false {
					// Add audiomapping options on the commanline
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(audio_stream_number_int))
				}
			}

			// Add color subsampling options to SD commandline if needed
			if color_subsampling != "yuv420p" && parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
				sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, color_subsampling_options...)
			}

			// Add audio compression options to SD commandline
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				if force_lossless.is_turned_on == true {

					// If main video audio is lossless use aac compression for the SD video
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-c:a", "aac", "-b:a", bitrate_str)

				} else {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, audio_compression_options...)
				}
			}

			// Add audiomapping options on the SD commanline
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				if no_audio.is_turned_on == false {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(audio_stream_number_int))
				}
			}

			if scale_to_sd.is_turned_on == false {
				if processing_duration.user_string != "" {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-t", processing_duration.user_string)
				}
			}

			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
				if processing_duration.user_string != "" {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-t", processing_duration.user_string)
				}
			}

			ffmpeg_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))
			ffmpeg_sd_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension) + "-sd")

			if temp_file_directory.user_string != "" {
				ffmpeg_2_pass_logfile_path = filepath.Join(temp_file_directory.user_string, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))
				ffmpeg_sd_2_pass_logfile_path = filepath.Join(temp_file_directory.user_string, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension) + "-sd")
			}

			if fast_encode.is_turned_on == false && crf_option.is_turned_on == false && scale_to_sd.is_turned_on == false {
				// Add 2 - pass logfile path to ffmpeg commandline
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-passlogfile")
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_2_pass_logfile_path)
			}

			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				if fast_encode.is_turned_on == false && crf_option.is_turned_on == false {
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-passlogfile")
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, ffmpeg_sd_2_pass_logfile_path)
				}
			}

			if scale_to_sd.is_turned_on == false {
				// Add video output format to ffmpeg commandline
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_video_format...)
			}

			// Add video output format SD to ffmpeg commandline
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
				sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, output_video_format...)
			}

			// Copy ffmpeg pass 2 commandline to ffmpeg pass 1 commandline
			ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, ffmpeg_pass_2_commandline...)
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
				sd_ffmpeg_pass_1_commandline = append(sd_ffmpeg_pass_1_commandline, sd_ffmpeg_pass_2_commandline...)
			}

			// Add pass 1/2 info on ffmpeg commandline
			if fast_encode.is_turned_on == false && crf_option.is_turned_on == false && scale_to_sd.is_turned_on == false {

				ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "-pass", "1")
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-pass", "2")

				// Add /dev/null output option to ffmpeg pass 1 commandline
				ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, "/dev/null")

			}

			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

				if fast_encode.is_turned_on == false && crf_option.is_turned_on == false {
					sd_ffmpeg_pass_1_commandline = append(sd_ffmpeg_pass_1_commandline, "-pass", "1")
					sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, "-pass", "2")

					// Add /dev/null output option to ffmpeg pass 1 commandline
					sd_ffmpeg_pass_1_commandline = append(sd_ffmpeg_pass_1_commandline, "/dev/null")
				}
			}

			// Add outfile path to ffmpeg pass 2 commandline
			if scale_to_sd.is_turned_on == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, output_file_absolute_path)
			}
			sd_ffmpeg_pass_2_commandline = append(sd_ffmpeg_pass_2_commandline, sd_output_file_absolute_path)

			// Add parallel SD compression options to FFmpeg commandline.
			if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {
				ffmpeg_pass_1_commandline = append(ffmpeg_pass_1_commandline, sd_ffmpeg_pass_1_commandline...)
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, sd_ffmpeg_pass_2_commandline...)
			}

			// If we have "fast" mode on then we will only do 1-pass encoding and the pass 1 commandline is the same as pass 2.
			// In this case we won't do pass 2 at all.
			if fast_encode.is_turned_on == true || crf_option.is_turned_on == true {
				ffmpeg_pass_1_commandline = ffmpeg_pass_2_commandline
				sd_ffmpeg_pass_1_commandline = sd_ffmpeg_pass_2_commandline
			}

			/////////////////////////////////////////
			// Print variable values in debug mode //
			/////////////////////////////////////////
			if debug_option.is_turned_on == true {
				fmt.Println()
				fmt.Println("video_compression_options_sd:", video_compression_options_sd)
				fmt.Println("video_compression_options_hd:", video_compression_options_hd)
				fmt.Println("main_video_compression_options:", main_video_compression_options)
				fmt.Println("audio_compression_options:", audio_compression_options)
				fmt.Println("denoise_options:", denoise_options)
				fmt.Println("deinterlace_options:", deinterlace_options)
				fmt.Println("ffmpeg_commandline_start:", ffmpeg_commandline_start)
				fmt.Println("subtitle_burn_number:", subtitle_burn_number)
				fmt.Println("subtitle_language_option.user_string:", subtitle_language_option.user_string)
				fmt.Println("subtitle_burn_vertical_offset_int:", subtitle_burn_vertical_offset_int)
				fmt.Println("subtitle_burn_downscale.is_turned_on:", subtitle_burn_downscale.is_turned_on)
				fmt.Println("subtitle_burn_palette.user_string:", subtitle_burn_palette.user_string)
				fmt.Println("subtitle_burn_split.is_turned_on:", subtitle_burn_split.is_turned_on)
				fmt.Println("subtitle_mux_bool:", subtitle_mux_bool)
				fmt.Println("user_subtitle_mux_numbers_slice:", user_subtitle_mux_numbers_slice)
				fmt.Println("user_subtitle_mux_languages_slice:", user_subtitle_mux_languages_slice)
				fmt.Println("grayscale_option.is_turned_on:", grayscale_option.is_turned_on)
				fmt.Println("grayscale_options:", grayscale_options)
				fmt.Println("color_subsampling_options", color_subsampling_options)
				fmt.Println("autocrop_option.is_turned_on:", autocrop_option.is_turned_on)
				fmt.Println("subtitle_burn_number:", subtitle_burn_number)
				fmt.Println("no_deinterlace.is_turned_on:", no_deinterlace.is_turned_on)
				fmt.Println("denoise_option.is_turned_on:", denoise_option.is_turned_on)
				fmt.Println("audio_stream_number_int:", audio_stream_number_int)
				fmt.Println("scan_mode_only.is_turned_on:", scan_mode_only.is_turned_on)
				fmt.Println("search_start_option.user_string", search_start_option.user_string)
				fmt.Println("processing_stop_time.user_string", processing_stop_time.user_string)
				fmt.Println("processing_duration.user_string", processing_duration.user_string)
				fmt.Println("fast_encode_and_search.is_turned_on", fast_encode_and_search.is_turned_on)
				fmt.Println("fast_search.is_turned_on", fast_search.is_turned_on)
				fmt.Println("fast_encode.is_turned_on", fast_encode.is_turned_on)
				fmt.Println("burn_timecode.is_turned_on", burn_timecode.is_turned_on)
				fmt.Println("timecode_burn_options", timecode_burn_options)
				fmt.Println("crf_option.is_turned_on", crf_option.is_turned_on)
				fmt.Println("crf_value", crf_value)
				fmt.Println("debug_option.is_turned_on", debug_option.is_turned_on)
				fmt.Println()
				fmt.Println("input_filenames:", input_filenames)
			}

			/////////////////////////////////////
			// Run Pass 1 encoding with FFmpeg //
			/////////////////////////////////////
			// Add pass 1 messages to processing log.
			pass_1_commandline_for_logfile := strings.Join(ffmpeg_pass_1_commandline, " ")

			// Make a copy of the FFmpeg commandline for writing in the logfile.
			// Modify commandline so that it works if the user wants to copy and paste it from the logfile and run it.
			// The filter command needs to be in single quotes '
			index := strings.Index(pass_1_commandline_for_logfile, "-filter_complex")
			first_part_of_string := pass_1_commandline_for_logfile[:index + 16]
			first_part_of_string = first_part_of_string + "'"

			second_part_of_string := pass_1_commandline_for_logfile[index + 16:]
			index = strings.Index(second_part_of_string, " -")
			third_part_of_string := second_part_of_string[index:]
			second_part_of_string = second_part_of_string[:index]
			second_part_of_string = second_part_of_string + "'"

			pass_1_commandline_for_logfile = first_part_of_string + second_part_of_string + third_part_of_string

			if debug_option.is_turned_on == true || only_print_commands.is_turned_on == true {

				fmt.Println()
				fmt.Println("ffmpeg_pass_1_commandline:")
				fmt.Println(pass_1_commandline_for_logfile)

			} else {
				fmt.Println()

				if inverse_telecine.is_turned_on == true {
					fmt.Print("Performing Inverse Telecine on video.\n")
				}

				if frame_rate_str == "29.970" && inverse_telecine.is_turned_on == false {
					fmt.Println("\033[7mWarning: Video frame rate is 29.970. You may need to pullup (Inverse Telecine) this video with option -it\033[0m")
				}

				if parallel_sd.is_turned_on == true || scale_to_sd.is_turned_on == true {

					if scale_to_sd.is_turned_on == false && crf_option.is_turned_on == true {

						fmt.Println("Encoding main and SD video with", main_video_2_pass_bitrate_str)

					} else if scale_to_sd.is_turned_on == true && crf_option.is_turned_on == true {

						fmt.Println("Encoding SD video with", main_video_2_pass_bitrate_str)

					} else if scale_to_sd.is_turned_on == true && crf_option.is_turned_on == false {

						fmt.Println("Encoding SD-video with bitrate:", sd_video_bitrate)

					} else {
						fmt.Println("Encoding main video with bitrate:", main_video_2_pass_bitrate_str)
						fmt.Println("Encoding SD-video with bitrate:", sd_video_bitrate)
					}
				} else {

					if crf_option.is_turned_on == true {
						fmt.Println("Encoding video with", main_video_2_pass_bitrate_str)
					} else {
						fmt.Println("Encoding video with bitrate:", main_video_2_pass_bitrate_str)
					}
				}

				if color_subsampling != "yuv420p" {
					fmt.Println("Subsampling color:", color_subsampling, "---> yuv420p")
				}

				if no_audio.is_turned_on == true {

					fmt.Println("Audio processing is off.")

				} else if audio_compression_ac3.is_turned_on == true {

					fmt.Printf("Encoding %s channel audio to ac3 with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])

				} else if audio_compression_aac.is_turned_on == true {

					fmt.Printf("Encoding %s channel audio to aac with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])

				} else if audio_compression_opus.is_turned_on == true {

					fmt.Printf("Encoding %s channel audio to opus with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])
				} else {

					fmt.Printf("Copying %s audio to target.\n", audio_codec)
				}

				fmt.Printf("Pass 1 encoding: " + inputfile_name + " ")
			}

			pass_1_start_time = time.Now()

			////////////////
			// Run FFmpeg //
			////////////////
			var ffmpeg_pass_1_output_temp []string
			var ffmpeg_pass_1_error_output_temp []string
			var error_code error

			if only_print_commands.is_turned_on == false {
				ffmpeg_pass_1_output_temp, ffmpeg_pass_1_error_output_temp, error_code = run_external_command(ffmpeg_pass_1_commandline)
			}

			if error_code != nil {

				fmt.Println("\n\nFFmpeg reported error:", "\n")

				if len(ffmpeg_pass_1_output_temp) != 0 {
					for _, textline := range ffmpeg_pass_1_output_temp {
						fmt.Println(textline)
					}
				}

				if len(ffmpeg_pass_1_error_output_temp) != 0 {
					for _, textline := range ffmpeg_pass_1_error_output_temp {
						fmt.Println(textline)
					}
				}

				os.Exit(1)
			}

			pass_1_elapsed_time = time.Since(pass_1_start_time)

			if only_print_commands.is_turned_on == false {
				fmt.Printf("took %s", pass_1_elapsed_time.Round(time.Millisecond))
			}
			fmt.Println()

			// Add pass 2 messages to processing log.
			pass_2_commandline_for_logfile := strings.Join(ffmpeg_pass_2_commandline, " ")

			index = strings.Index(pass_2_commandline_for_logfile, "-filter_complex")
			first_part_of_string = pass_2_commandline_for_logfile[:index + 16]
			first_part_of_string = first_part_of_string + "'"

			second_part_of_string = pass_2_commandline_for_logfile[index + 16:]
			index = strings.Index(second_part_of_string, " -")
			third_part_of_string = second_part_of_string[index:]
			second_part_of_string = second_part_of_string[:index]
			second_part_of_string = second_part_of_string + "'"

			pass_2_commandline_for_logfile = first_part_of_string + second_part_of_string + third_part_of_string

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 1 Options:")
			log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
			log_messages_str_slice = append(log_messages_str_slice, pass_1_commandline_for_logfile)

			if debug_option.is_turned_on == true {

				fmt.Println()

				ffmpeg_pass_1_output := strings.TrimSpace(strings.Join(ffmpeg_pass_1_output_temp, ""))

				if len(ffmpeg_pass_1_output) > 0 {
					fmt.Println("Length of FFmpeg Pass 1 Text Output", len(ffmpeg_pass_1_output))
					fmt.Println(ffmpeg_pass_1_output)
				}

				if error_code != nil {
					fmt.Println(ffmpeg_pass_1_output_temp)
				}
			}

			/////////////////////////////////////
			// Run Pass 2 encoding with FFmpeg //
			/////////////////////////////////////
			if fast_encode.is_turned_on == false && crf_option.is_turned_on == false {

				if debug_option.is_turned_on == true || only_print_commands.is_turned_on == true  {

					fmt.Println()
					fmt.Println("ffmpeg_pass_2_commandline:")
					fmt.Println(pass_2_commandline_for_logfile)

				} else {

					pass_2_elapsed_time = time.Since(start_time)
					fmt.Printf("Pass 2 encoding: " + inputfile_name + " ")
				}

				if only_print_commands.is_turned_on == true {
					os.Exit(0)
				}

				pass_2_start_time = time.Now()

				////////////////
				// Run FFmpeg //
				////////////////
				ffmpeg_pass_2_output_temp, ffmpeg_pass_2_error_output_temp, error_code := run_external_command(ffmpeg_pass_2_commandline)

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", "\n")

					if len(ffmpeg_pass_2_output_temp) != 0 {
						for _, textline := range ffmpeg_pass_2_output_temp {
							fmt.Println(textline)
						}
					}

					if len(ffmpeg_pass_2_error_output_temp) != 0 {
						for _, textline := range ffmpeg_pass_2_error_output_temp {
							fmt.Println(textline)
						}
					}

					os.Exit(1)
				}

				pass_2_elapsed_time = time.Since(pass_2_start_time)
				fmt.Printf("took %s", pass_2_elapsed_time.Round(time.Millisecond))
				fmt.Println()

				if split_video == true {
					fmt.Println("\nPlease check the following cut points for possible video / audio glitches and adjust split times if needed: ")

					for _, timecode := range cut_positions_as_timecodes {
						fmt.Println(timecode)
					}
				}

				fmt.Println()

				log_messages_str_slice = append(log_messages_str_slice, "")
				log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 2 Options:")
				log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
				log_messages_str_slice = append(log_messages_str_slice, pass_2_commandline_for_logfile)

				if debug_option.is_turned_on == true {

					fmt.Println()

					ffmpeg_pass_2_output := strings.TrimSpace(strings.Join(ffmpeg_pass_2_output_temp, ""))

					if len(ffmpeg_pass_2_output) > 0 {
						fmt.Println("Length of FFmpeg Pass Text 2 Output", len(ffmpeg_pass_2_output))
						fmt.Println(ffmpeg_pass_2_output)
					}

					if ffmpeg_pass_2_output_temp != nil {
						fmt.Println(ffmpeg_pass_2_output_temp)
					}

					fmt.Println()
				}
			}


			if only_print_commands.is_turned_on == true {
				os.Exit(0)
			}

			////////////////////////////
			// Remove temporary files //
			////////////////////////////

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log")
			}

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log.mbtree"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log.mbtree")
			}

			if _, err := os.Stat(ffmpeg_sd_2_pass_logfile_path + "-0.log"); err == nil {
				os.Remove(ffmpeg_sd_2_pass_logfile_path + "-0.log")
			}

			if _, err := os.Stat(ffmpeg_sd_2_pass_logfile_path + "-0.log.mbtree"); err == nil {
				os.Remove(ffmpeg_sd_2_pass_logfile_path + "-0.log.mbtree")
			}

			if debug_option.is_turned_on == true {

				fmt.Println("\nSplitfiles are not deleted in debug - mode.\n")

			} else {

				for _, splitfile_name := range list_of_splitfiles {
					if _, err := os.Stat(splitfile_name); err == nil {
						os.Remove(splitfile_name)
					} else {
						fmt.Println("Could not delete splitfile:", splitfile_name)
					}
				}

				if _, err := os.Stat(split_info_file_absolute_path); !os.IsNotExist(err) {
					if err = os.Remove(split_info_file_absolute_path); err != nil {
						fmt.Println("Could not delete split_info_file:", split_info_file_absolute_path)
					}
				}

			}

			if subtitle_burn_split.is_turned_on == true {
				if debug_option.is_turned_on == true {
					fmt.Println("\nExtracted subtitle images are not deleted in debug - mode.\n")
				} else {

					// Remove subtitle directories.
					if _, err := os.Stat(original_subtitles_absolute_path); err == nil {
						os.RemoveAll(original_subtitles_absolute_path)
					}

					if _, err := os.Stat(fixed_subtitles_absolute_path); err == nil {
						os.RemoveAll(fixed_subtitles_absolute_path)
					}

					// Delete subtitle dir if it is empty
					file_handle, err := os.Open(subtitle_extract_base_path)
					defer file_handle.Close()

					if err == nil {
						_, dir_empty := file_handle.Readdirnames(1)

						if dir_empty == io.EOF {
							os.Remove(subtitle_extract_base_path)
						}
					}
				}
			}

			// Delete SD output directory if it is empty
			if parallel_sd.is_turned_on == true {
				file_handle, err := os.Open(sd_directory_path)
				defer file_handle.Close()

				if err == nil {
					_, dir_empty := file_handle.Readdirnames(1)

					if dir_empty == io.EOF {
						os.Remove(sd_directory_path)
					}
				}
			}

			elapsed_time := time.Since(start_time)
			fmt.Printf("All processing took %s", elapsed_time.Round(time.Millisecond))
			fmt.Println()

			// Add messages to processing log.
			log_messages_str_slice = append(log_messages_str_slice, "")
			pass_1_elapsed_time := pass_1_elapsed_time.Round(time.Millisecond)
			pass_2_elapsed_time := pass_2_elapsed_time.Round(time.Millisecond)
			total_elapsed_time := elapsed_time.Round(time.Millisecond)
			log_messages_str_slice = append(log_messages_str_slice, "Pass 1 took: "+pass_1_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "Pass 2 took: "+pass_2_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "All processing took: "+total_elapsed_time.String())

			if split_video == true {
				log_messages_str_slice = append(log_messages_str_slice, "\nPlease check the following edit positions for video / audio glitches: ")

				for _, timecode := range cut_positions_as_timecodes {
					log_messages_str_slice = append(log_messages_str_slice, timecode)
				}
			}

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

