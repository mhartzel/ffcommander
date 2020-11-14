package main

import (
	"bytes"
	"bufio"
	"crypto/md5"
	"flag"
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
var version_number string = "2.01" // This is the version of this program
var Complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)

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
			fmt.Println(filename, "is part of package ImageMagick and is needed by the -sp functionality.")
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

func process_split_times(split_times *string, debug_mode_on *bool) ([]string, []string) {

	var cut_list_seconds_str_slice, cut_list_positions_and_durations_seconds, cut_positions_after_processing_seconds, cut_positions_as_timecodes []string
	var seconds_total_str, error_happened string

	cut_list_string_slice := strings.Split(*split_times, ",")

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

	if *debug_mode_on == true {
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

			if *debug_mode_on == true {
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

	// Convert second to timecode values
	cut_positions_as_timecodes = convert_cut_positions_to_timecode(cut_positions_after_processing_seconds)

	if *debug_mode_on == true {
		fmt.Println("process_split_times: split_times:", *split_times)
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

func subtitle_trim(original_subtitles_absolute_path string, fixed_subtitles_absolute_path string, files_str_slice []string, video_width string, video_height string, process_number int, return_channel chan int, subtitle_burn_resize string) {

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

		subtitle_trim_commandline = append(subtitle_trim_commandline, "magick", filepath.Join(original_subtitles_absolute_path, subtitle_name), "-trim", "-print", "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]", "-compress", "rle", filepath.Join(fixed_subtitles_absolute_path, subtitle_name))

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

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	//////////////////////////////////////////
	// Define and parse commandline options //
	//////////////////////////////////////////
	// Audio options
	var audio_language_str = flag.String("a", "", "Audio language: -a fin or -a eng or -a ita  Find audio stream corresponding the language code. Only use option -an or -a not both.")
	var audio_stream_number_int = flag.Int("an", 0, "Audio stream number, -a 1. Only use option -an or -a not both.")
	var audio_compression_ac3 = flag.Bool("ac3", false, "Compress audio as ac3. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate. 6 channels uses the ac3 max bitrate of 640k.")
	var audio_compression_aac = flag.Bool("aac", false, "Compress audio as aac. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.")
	var audio_compression_opus = flag.Bool("opus", false, "Compress audio as opus. Opus support in mp4 container is experimental as of FFmpeg vesion 4.2.1. Bitrate of 128k is used for each audio channel meaning 2 channels is compressed using 256k bitrate, 6 channels uses 768k bitrate.")
	var audio_compression_flac = flag.Bool("flac", false, "Compress audio in lossless Flac - format")
	var no_audio = flag.Bool("na", false, "Disable audio processing. The resulting file will have no audio, only video.")

	// Video options
	var autocrop_bool = flag.Bool("ac", false, "Autocrop. Find crop values automatically by doing 10 second spot checks in 10 places for the duration of the file.")
	var denoise_bool = flag.Bool("dn", false, "Denoise. Use HQDN3D - filter to remove noise from the picture. This option is equal to Hanbrakes 'medium' noise reduction settings.")
	var grayscale_bool = flag.Bool("gr", false, "Convert video to Grayscale. Use this option if the original source is black and white. This results more bitrate being available for b/w information and better picture quality.")
	var force_hd_bool = flag.Bool("hd", false, "Force video encoding to use HD bitrate and profile (Profile = High, Level = 4.1, Bitrate = 8000k) By default this program decides video encoding profile and bitrate automatically depending on the vertical resolution of the picture.")
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace. By default deinterlace is always used. This option disables it.")
	var split_times = flag.String("sf", "", "Split out parts of the file. Give colon separated start and stop times for the parts of the file to use, for example: -sf 0,10:00,01:35:12.800,01:52:14 defines that 0 secs - 10 mins of the start of the file will be used and joined to the next part that starts at 01 hours 35 mins 12 seconds and 800 milliseconds and stops at 01 hours 52 mins 14 seconds. Don't use space - characters. A zero or word 'start' can be used to mark the absolute start of the file and word 'end' the end of the file. Both start and stop times must be defined.")
	var burn_timecode_bool = flag.Bool("tc", false, "Burn timecode on top of the video. Timecode can be used to look for exact edit points for the file split feature")
	var inverse_telecine = flag.Bool("it", false, "Perform inverse telecine on 29.97 fps material to return it back to original 24 fps.")

	// Options that affect both video and audio
	var force_lossless_bool = flag.Bool("ls", false, "Force encoding to use lossless 'utvideo' compression for video and 'flac' compression for audio. This also turns on -fe")

	// Subtitle options
	var subtitle_burn_language_str = flag.String("s", "", "Subtitle language: -s fin or -s eng -s ita  Only use option -sn or -s not both. This option affects only subtitle burned on top of video.")
	var subtitle_burn_downscale = flag.Bool("sd", false, "Subtitle `downscale`. When cropping video widthwise, scale down subtitle to fit on top of the cropped video instead of cropping the subtitle. This option results in smaller subtitle font. This option affects only subtitle burned on top of video.")
	var subtitle_burn_int = flag.Int("sn", -1, "Subtitle stream `number, -sn 1` Use subtitle number 1 from the source file. Only use option -sn or -s not both. This option affects only subtitle burned on top of video.")
	var subtitle_burn_vertical_offset_int = flag.Int("so", 0, "Subtitle `offset`, -so 55 (move subtitle 55 pixels down), -so -55 (move subtitle 55 pixels up). This option affects only subtitle burned on top of video.")
	var user_subtitle_mux_languages_str = flag.String("sm", "", "Mux subtitle into the target file. This only works with dvd, dvb and bluray bitmap based subtitles. If this option is not set then subtitles will be burned into the video. This option can not be used by itself, it must be used with -s or -sn. mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file.")
	var user_subtitle_mux_numbers_str = flag.String("smn", "", "Mux subtitle into the target file. This only works with dvd, dvb and bluray bitmap based subtitles. If this option is not set then subtitles will be burned into the video. This option can not be used by itself, it must be used with -s or -sn. mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file.")
	var subtitle_burn_palette = flag.String("palette", "", "Hack dvd subtitle color palette. Option takes 1-16 comma separated hex numbers ranging from 0 to f. Zero = black, f = white, so only shades between black -> gray -> white can be defined. FFmpeg requires 16 hex numbers, so f's are automatically appended to the end of user given numbers. Each dvd uses color mapping differently so you need to try which numbers control the colors you want to change. Usually the first 4 numbers control the colors. Example: -palette f,0,f  This option affects only subtitle burned on top of video.")
	var subtitle_burn_split = flag.Bool("sp", false, "Subtitle Split. Move subtitles that are above the center of the screen up to the top of the screen and subtitles below center down on the bottom of the screen. Distance from the screen edge will be picture height divided by 100 and rounded down to nearest integer. Minimum distance is 5 pixels and max 20 pixels. Subtitles will be automatically centered horizontally. You can resize subtitles with the -sr option when usind Subtitle Split. This option requires installing ImageMacick. This option affects only subtitle burned on top of video.")
	var subtitle_burn_resize = flag.String("sr", "", "Subtitle Resize. Values less than 1 makes subtitles smaller, values bigger than 1 makes subtitle larger. This option can only be user with the -sp option. Example: make subtitle 25% smaller: -sr 0.75   make subtitle 50% smaller: -sr 0.50   make subtitle 75% larger: -sr 1.75. This option affects only subtitle burned on top of video.")

	// Scan options
	var fast_bool = flag.Bool("f", false, "This is the same as using options -fs and -fe at the same time.")
	var fast_encode_bool = flag.Bool("fe", false, "Fast encoding mode. Encode video using 1-pass encoding.")
	var fast_search_bool = flag.Bool("fs", false, "Fast seek mode. When using the -fs option with -st do not decode video before the point we are trying to locate, but instead try to jump directly to it. This search method might or might not be accurate depending on the file format.")
	var scan_mode_only_bool = flag.Bool("scan", false, "Only scan input file and print video and audio stream info.")
	var search_start_str = flag.String("st", "", "Start time. Start video processing from this timecode. Example -st 30:00 starts processing from 30 minutes from the start of the file.")
	var processing_stop_time_str = flag.String("et", "", "End time. Stop video processing to this timecode. Example -et 01:30:00 stops processing at 1 hour 30 minutes. You can define a time range like this: -st 10:09 -et 01:22:49.500 This results in a video file that starts at 10 minutes 9 seconds and stops at 1 hour 22 minutes, 49 seconds and 500 milliseconds.")
	var processing_time_str = flag.String("d", "", "Duration of video to process. Example -d 01:02 process 1 minutes and 2 seconds of the file. Use either -et or -d option not both.")

	// Misc options
	var debug_mode_on = flag.Bool("debug", false, "Turn on debug mode and show info about internal variables and the FFmpeg commandlines used.")
	var use_matroska_container = flag.Bool("mkv", false, "Use matroska (mkv) as the output file wrapper format.")
	var show_program_version_short = flag.Bool("v", false, "Show the version of this program.")
	var show_program_version_long = flag.Bool("version", false, "Show the version of this program.")
	var temp_file_directory = flag.String("td", "", "Path to directory for temporary files, example_ -td PathToDir. This option directs temporary files created with 2-pass encoding and subtitle processing with the -sp switch to a separate directory. If the temp dir is a ram or a fast ssd disk then it speeds up processing with the -sp switch. Processing files with the -sp switch extracts every frame of the movie as a picture, so you need to have lots of space in the temp directory. For a FullHD movie you need to have 20 GB or more free storage. If you run multiple instances of this program simultaneously each instance processing one FullHD movie then you need 20 GB or more free storage for each movie that is processed at the same time. -sp switch extracts movie subtitle frames with FFmpeg and FFmpeg fails silently if it runs out of storage space. If this happens then some of the last subtitles won't be available when the video is compressed and this results the last available subtitle to be 'stuck' on top of video to the end of the movie.")

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
	var video_bitrate string
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
	subtitle_extract_dir := "subtitles"
	original_subtitles_dir := "original_subtitles"
	fixed_subtitles_dir := "fixed_subtitles"

	///////////////////////////////
	// Parse commandline options //
	///////////////////////////////
	flag.Parse()

	// The unparsed options left on the commandline are filenames, store them in a slice.
	for _, file_name := range flag.Args() {

		inputfile_full_path,_ := filepath.Abs(file_name)
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

	/////////////////////////////////////////////////////////
	// Test if needed executables can be found in the path //
	/////////////////////////////////////////////////////////
	find_executable_path("ffmpeg")
	find_executable_path("ffprobe")

	if *subtitle_burn_split == true {
		find_executable_path("magick") // Starting from ImageMagick 7 the "magick" command should be used instead of the "convert" - command.
		find_executable_path("mogrify")
		os.Setenv("MAGICK_THREAD_LIMIT", "1") // Disable ImageMagick multithreading it only makes processing slower. This sets an environment variable in the os.
	}

	// Test that user gave a string not a number for options -a and -s
	if _, err := strconv.Atoi(*audio_language_str); err == nil {
		fmt.Println()
		fmt.Println("The option -a requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	if _, err := strconv.Atoi(*subtitle_burn_language_str); err == nil {
		fmt.Println()
		fmt.Println("The option -s requires a language code like: eng, fin, ita not a number.")
		fmt.Println()
		os.Exit(0)
	}

	if *subtitle_burn_resize != "" && *subtitle_burn_split == false {
		fmt.Println()
		fmt.Println("Subtitle resize can only be used with the -sp option, not alone.")
		fmt.Println()
		os.Exit(0)
	}

	// Test if user gave a valid float on the commandline
	if *subtitle_burn_resize != "" {

		subtitle_resize_float, float_parse_error := strconv.ParseFloat(*subtitle_burn_resize, 64)

		if  float_parse_error != nil || subtitle_resize_float == 0.0 {

			fmt.Println("Error:", *subtitle_burn_resize, "is not a valid number.")
			os.Exit(1)
		}
	}

	// Convert time values used in splitting the inputfile to seconds
	if *split_times != "" {
		split_video = true
		*use_matroska_container = true
		cut_list_seconds_str_slice, cut_positions_as_timecodes = process_split_times(split_times, debug_mode_on)
	}

	// -f option turns on both options -fs and -fe
	if *fast_bool == true {
		*fast_search_bool = true
		*fast_encode_bool = true
	}

	// Convert processing end time to duration and store it in variable used with -d option (duration).
	// FFmpeg does not understarnd end times, only start time + duration.
	if *processing_stop_time_str != "" {

		start_time := ""
		end_time := ""
		error_happened := ""

		if *search_start_str != "" {
			start_time, error_happened = convert_timecode_to_seconds(*search_start_str)

			if error_happened != "" {
				fmt.Println("\nError when converting start time to seconds: " + error_happened + "\n")
				os.Exit(1)
			}
		}

		start_time_int, atoi_error := strconv.Atoi(start_time)

		if atoi_error != nil {
			fmt.Println()
			fmt.Println("Error converting start time", *search_start_str, "to integer")
			fmt.Println()
			os.Exit(0)
		}

		end_time, error_happened = convert_timecode_to_seconds(*processing_stop_time_str)

		if error_happened != "" {
			fmt.Println("\nError when converting end time to seconds: " + error_happened + "\n")
			os.Exit(1)
		}

		end_time_int, atoi_error := strconv.Atoi(end_time)

		if atoi_error != nil {
			fmt.Println()
			fmt.Println("Error converting end time", *processing_stop_time_str, "to integer")
			fmt.Println()
			os.Exit(0)
		}

		duration_int := end_time_int - start_time_int
		*processing_time_str = convert_seconds_to_timecode(strconv.Itoa(duration_int))
	}

	// Disable -st and -d options if user did use the -sf option and input some edit times
	if split_video == true {
		*search_start_str = ""
		*processing_time_str = ""
	}

	// Always use 1-pass encoding with lossless encoding. Turn on option -fe.
	if *force_lossless_bool == true {
		*fast_encode_bool = true
	}

	if *subtitle_burn_split == true && *search_start_str != "" && *fast_bool == false {
		fmt.Println("\nOptions -st -sp and 2-pass encoding won't work correctly together.")
		fmt.Println("You options are: disable 2-pass encoding with the -f option or don't use the -st option.\n")
		os.Exit(1)
	}

	// Check dvd palette hacking option string correctness.
	if *subtitle_burn_palette != "" {
		temp_slice := strings.Split(*subtitle_burn_palette, ",")
		*subtitle_burn_palette = ""
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

			*subtitle_burn_palette = *subtitle_burn_palette + strings.Repeat(strings.ToLower(character), 6)

			if counter < len(temp_slice)-1 {
				*subtitle_burn_palette = *subtitle_burn_palette + ","
			}

		}

		if len(temp_slice) < 16 {

			*subtitle_burn_palette = *subtitle_burn_palette + ","

			for counter := len(temp_slice); counter < 16; counter++ {
				*subtitle_burn_palette = *subtitle_burn_palette + "ffffff"

				if counter < 15 {
					*subtitle_burn_palette = *subtitle_burn_palette + ","
				}
			}
		}
	}

	// Parse subtitle list
	var user_subtitle_mux_numbers_slice []string
	var user_subtitle_mux_languages_slice []string
	subtitle_mux_bool := false
	subtitle_burn_bool := false
	highest_subtitle_number := ""

	if *user_subtitle_mux_numbers_str != "" && *user_subtitle_mux_languages_str != "" {
		fmt.Println()
		fmt.Println("Error, only -sm or -smn can be used at a time.")
		fmt.Println()
		os.Exit(0)
	}

	if *user_subtitle_mux_numbers_str != "" {

		user_subtitle_mux_numbers_slice = strings.Split(*user_subtitle_mux_numbers_str, ",")

		if len(user_subtitle_mux_numbers_slice) == 0 {
			fmt.Println()
			fmt.Println("Error parsing subtitle numbers from: ", *user_subtitle_mux_numbers_str)
			fmt.Println()
			os.Exit(0)
		}

		// Check that user gave only numbers for the option and store highest subtitle number
		for _, number := range user_subtitle_mux_numbers_slice {

			highest_subtitle_number_int ,_ := strconv.Atoi(highest_subtitle_number)

			if number_int, atoi_error := strconv.Atoi(number) ; atoi_error != nil {
				fmt.Println()
				fmt.Println("Error parsing subtitle number:", number, "in:", *user_subtitle_mux_numbers_str)
				fmt.Println()
				os.Exit(0)

			} else if number_int > highest_subtitle_number_int {
				highest_subtitle_number = number
			}
		}

		subtitle_mux_bool = true
	}

	if *user_subtitle_mux_languages_str != "" {

		user_subtitle_mux_languages_slice = strings.Split(*user_subtitle_mux_languages_str, ",")

		if len(user_subtitle_mux_languages_slice) == 0 {
			fmt.Println()
			fmt.Println("Error parsing subtitle languages from: ", *user_subtitle_mux_languages_str)
			fmt.Println()
			os.Exit(0)
		}

		subtitle_mux_bool = true
	}

	subtitle_burn_number := *subtitle_burn_int

	if *subtitle_burn_language_str != "" || subtitle_burn_number  != -1 {
		subtitle_burn_bool = true
	}

	if subtitle_mux_bool == true && subtitle_burn_bool == true {
		fmt.Println()
		fmt.Println("Error, you can only burn a subtitle on video or mux subtitles to the file, not both at the same time.")
		fmt.Println()
		os.Exit(0)
	}

	// Use the first subtitle if user wants subtitle split but did not specify subtitle number
	if *subtitle_burn_split == true && subtitle_burn_number == -1 {
		subtitle_burn_number = 0
	}

	if *debug_mode_on == true {

		fmt.Println()
		fmt.Println("Subtitle numbers:")
		fmt.Println("-----------------")

		for _, rivi := range user_subtitle_mux_numbers_slice {
			fmt.Println(rivi)
		}

		fmt.Println("Highest_subtitle_number", highest_subtitle_number)

		fmt.Println()

		fmt.Println("Subtitle languages")
		fmt.Println("------------------")

		for _, rivi := range user_subtitle_mux_languages_slice {
			fmt.Println(rivi)
		}

		fmt.Println()
	}

	// Check if user given path to temp folder exists
	if *temp_file_directory != "" {

		if _, err := os.Stat(*temp_file_directory); os.IsNotExist(err) {
			fmt.Println()
			fmt.Println("Path to temp file dir: ", *temp_file_directory, " does not exist.")
			fmt.Println()
			os.Exit(0)
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

	//////////////////////////////////////////////////
	// Define default processing options for FFmpeg //
	//////////////////////////////////////////////////
	video_compression_options_sd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "main", "-level", "4.0", "-b:v", "1600k"}
	video_compression_options_hd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "4.1", "-b:v", "8000k"}
	video_compression_options_lossless := []string{"-c:v", "utvideo"}
	audio_compression_options := []string{"-acodec", "copy"}
	audio_compression_options_lossless := []string{"-acodec", "flac"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}
	color_subsampling_options := []string{"-pix_fmt", "yuv420p"}
	var ffmpeg_commandline_start []string
	subtitle_stream_image_format := "tiff" // FFmpeg png extract is 30x slower than tiff, thats why we default to tiff.

	// Determine output file container
	output_video_format := []string{"-f", "mp4"}
	output_mp4_filename_extension := ".mp4"
	output_matroska_filename_extension := ".mkv"
	output_filename_extension := output_mp4_filename_extension
	output_matroska_wrapper_format := "matroska"

	if *force_lossless_bool == true || *use_matroska_container == true {
		// Use matroska as the output file wrapper format
		output_video_format = nil
		output_video_format = append(output_video_format, "-f", output_matroska_wrapper_format)
		output_filename_extension = output_matroska_filename_extension
	}

	if *no_deinterlace_bool == true {
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

	number_of_threads_to_use_for_video_compression := "auto"

	// Don't use more than 8 threads to process a single video because it is claimed that video quality goes down when using more than 10 cores with x264 encoder.
	// https://streaminglearningcenter.com/blogs/ffmpeg-command-threads-how-it-affects-quality-and-performance.html
	// Yllä mainitun sivun screenshot löytyy koodin hakemistosta '00-vanhat' nimellä: 'FFmpeg Threads Command How it Affects Quality and Performance.jpg'
	// It is said that this is caused by the fact that the processing threads can't use the results from other threads to optimize compression.
	// Also processing goes sligthly faster when ffmpeg is using max 8 cores.
	// The user may process files in 4 simultaneously by dividing videos to 4 dirs and processing each simultaneously.
	if number_of_physical_processors > 8 {
		number_of_threads_to_use_for_video_compression = "8"
	}

	if *debug_mode_on == true {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-hide_banner", "-threads", number_of_threads_to_use_for_video_compression)
	} else {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-loglevel", "level+error", "-threads", number_of_threads_to_use_for_video_compression)
	}

	if *subtitle_burn_split == true && *search_start_str != "" {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "-fflags", "+genpts")
	}

	///////////////////////////////
	// Scan inputfile properties //
	///////////////////////////////

	for _, inputfile_full_path := range input_filenames {

		// Get video info with: ffprobe -loglevel level+error -show_entries format:stream -print_format flat -i InputFile
		command_to_run_str_slice = nil
		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe", "-loglevel", "level+error", "-show_entries", "format:stream", "-print_format", "flat", "-i")

		if *debug_mode_on == true {
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

	if *debug_mode_on == true {

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
	if *scan_mode_only_bool == true {

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
		if *audio_language_str != "" {

			for audio_stream_number, audio_info := range audio_slice {
				audio_language = audio_info[0]

				if *audio_language_str == audio_language {
					*audio_stream_number_int = audio_stream_number
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

				error_messages = append(error_messages, "Error, file does not have audio language: " + *audio_language_str)
				error_messages_map[inputfile_full_path] = error_messages
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Audio: %s was found in audio stream number: %d\n", *audio_language_str, *audio_stream_number_int)
				fmt.Println()
			}

		} else {

			if len(audio_slice) - 1 < *audio_stream_number_int {

				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have an audio stream number: " + strconv.Itoa(*audio_stream_number_int))
				error_messages_map[inputfile_full_path] = error_messages

			} else {

				audio_info := audio_slice[*audio_stream_number_int]
				number_of_audio_channels = audio_info[2]
				audio_codec = strings.ToLower(audio_info[4])
			}
		}

		if audio_codec == "ac-3" {
			audio_codec = "ac3"
		}

		if *audio_compression_aac == true {
			audio_codec = "aac"
		}

		if *audio_compression_opus == true {
			audio_codec = "opus"
		}

		if *audio_compression_ac3 == true {
			audio_codec = "ac3"
		}

		if *force_lossless_bool == true {
			audio_codec = "flac"
		}

		if *audio_compression_flac == true {
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
		if *use_matroska_container == false && audio_stream_found == true {

			if audio_codec != "aac" && audio_codec != "ac3" && audio_codec != "mp2" && audio_codec != "mp3" && audio_codec != "dts" && audio_codec != "opus" {

				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, audio codec: " + audio_codec + " in file is not compatible with the mp4 wrapper format.")
				error_messages = append(error_messages, "Either use a compatible audio format (aac, ac3, mp2, mp3, dts) or the -mkv switch to export to a matroska file.")
				error_messages = append(error_messages, "")
				error_messages_map[inputfile_full_path] = error_messages
			}
		}

		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// If user gave us the subtitle language (fin, eng, ita) to burn on top of video, find the corresponding subtitle stream number //
		// If no matching subtitle is found stop the program.                                                                           //
		//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		if subtitle_burn_bool == true && *subtitle_burn_language_str != "" {


			subtitle_found := false

			for counter, subtitle_info := range subtitle_slice {
				subtitle_language = subtitle_info[0]

				if *subtitle_burn_language_str == subtitle_language {
					subtitle_burn_number = counter
					subtitle_found = true
					break // Continue searching the next file when the first matching subtitle has been found.
				}

			}

			if subtitle_found == false {
				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have subtitle language: " + *subtitle_burn_language_str)
				error_messages_map[inputfile_full_path] = error_messages
			}

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Printf("Subtitle: %s was found in file %s as number %s\n", *subtitle_burn_language_str, inputfile_full_path, subtitle_burn_number)
				fmt.Println()
			}

		} else if subtitle_burn_bool == true && subtitle_burn_number != -1 {

			// If user gave subtitle stream number, check that we have at least that much subtitle streams in the source file.
			if len(subtitle_slice) - 1 < subtitle_burn_number {

				var error_messages []string

				if _, item_found := error_messages_map[inputfile_full_path]; item_found == true {
					error_messages = error_messages_map[inputfile_full_path]
				}

				error_messages = append(error_messages, "Error, file does not have an subtitle stream number: " + strconv.Itoa(subtitle_burn_number))
				error_messages_map[inputfile_full_path] = error_messages
			}
		}

		///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// If user gave us the subtitle language (fin, eng, ita) to mux into the file, find the corresponding subtitle stream number //
		// If no matching subtitle is found stop the program.                                                                        //
		///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

		subtitle_found := false
		subtitle_type := ""

		if subtitle_mux_bool == true && len(user_subtitle_mux_languages_slice) > 0 {

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

				if *debug_mode_on == true {
					fmt.Println()
					fmt.Printf("Subtitle: %s was found in file %s\n", user_sub_language, inputfile_full_path)
					fmt.Println()
				}

				// Test if output subtitle type is compatible with the mp4 wrapper format
				if *use_matroska_container == false && subtitle_type == "hdmv_pgs_subtitle" {

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

		// Store info about selected video  always stream 0), audio and subtitle streams.
		if len(error_messages_map) == 0 {
			var selected_streams_temp []string
			selected_streams_temp = append(selected_streams_temp, "0", strconv.Itoa(*audio_stream_number_int), strconv.Itoa(subtitle_burn_number))
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
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension)+output_filename_extension)
		subtitle_extract_base_path := filepath.Join(inputfile_path, output_directory_name, subtitle_extract_dir)

		if *temp_file_directory != "" {
			subtitle_extract_base_path = filepath.Join(*temp_file_directory, output_directory_name, subtitle_extract_dir)
		}

		original_subtitles_absolute_path := filepath.Join(subtitle_extract_base_path, inputfile_name + "-" + original_subtitles_dir)
		fixed_subtitles_absolute_path := filepath.Join(subtitle_extract_base_path, inputfile_name + "-" + fixed_subtitles_dir)

		if *debug_mode_on == true {
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

		// Print information about processing
		file_counter = file_counter + 1
		file_counter_str = strconv.Itoa(file_counter)

		fmt.Println("")
		fmt.Println(strings.Repeat("#", 80))
		fmt.Println("")
		fmt.Println("Processing file " + file_counter_str + "/" + files_to_process_str + "  '" + inputfile_name + "'")

		audio_slice := file_info_slice[1]
		audio_info := audio_slice[*audio_stream_number_int]
		number_of_audio_channels = audio_info[2]
		audio_codec = audio_info[4]


		selected_streams_slice := selected_streams[inputfile_full_path]
		*audio_stream_number_int, _ = strconv.Atoi(selected_streams_slice[1])
		subtitle_burn_number, _ = strconv.Atoi(selected_streams_slice[2])

		////////////////////////////////////////////////////
		// Split out and use only some parts of the video //
		////////////////////////////////////////////////////
		if split_video == true {

			file_split_start_time = time.Now()
			counter_2 := 0
			list_of_splitfiles = nil

			// Open split_infofile for appending info about file splits
			split_info_filename = "00-splitfile_info.txt"
			split_info_file_absolute_path = filepath.Join(inputfile_path, output_directory_name, split_info_filename)

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

			audio_codec = "flac"

			for counter := 0; counter < len(cut_list_seconds_str_slice); counter = counter + 2 {
				counter_2++
				splitfile_name := "splitfile-" + strconv.Itoa(counter_2) + output_matroska_filename_extension
				split_file_path := filepath.Join(inputfile_path, output_directory_name, splitfile_name)
				list_of_splitfiles = append(list_of_splitfiles, split_file_path)

				ffmpeg_file_split_commandline = nil
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, ffmpeg_commandline_start...)
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-i", inputfile_full_path, "-ss", cut_list_seconds_str_slice[counter])

				// There is no timecode if the user wants to process to the end of file. Skip the -t FFmpeg option since FFmpeg processes to the end of file without it.
				if len(cut_list_seconds_str_slice)-1 > counter {
					ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-t", cut_list_seconds_str_slice[counter+1])
				}

				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-vcodec", "utvideo", "-acodec", "flac", "-scodec", "copy", "-map", "0", split_file_path)

				fmt.Println("Creating splitfile: " + splitfile_name)
				log_messages_str_slice = append(log_messages_str_slice, strings.Join(ffmpeg_file_split_commandline, " "))

				// Write split file names to a text file
				if _, err = split_info_file_pointer.WriteString("file " + splitfile_name + "\n"); err != nil {
					fmt.Println("")
					fmt.Println("Error, could not write to split info file:", split_info_filename)
					log.Fatal(err)
					os.Exit(0)
				}

				file_split_output_temp, file_split_error_output_temp, error_code := run_external_command(ffmpeg_file_split_commandline)

				if *debug_mode_on == true {
					fmt.Println(ffmpeg_file_split_commandline, "\n")
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

			file_split_elapsed_time = time.Since(file_split_start_time)
			fmt.Printf("\nSplitfile creation took %s\n", file_split_elapsed_time.Round(time.Millisecond))
			fmt.Println()

			log_messages_str_slice = append(log_messages_str_slice, "\nSplitfile creation took "+file_split_elapsed_time.Round(time.Millisecond).String())

			// If user has not defined audio output codec use aac rather than flac
			if *audio_compression_flac != true && audio_codec == "flac" {
				*audio_compression_aac = true
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

		if *autocrop_bool == true {

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

			user_defined_search_start_str = *search_start_str

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

			user_defined_video_duration_str = *processing_time_str

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

			if *debug_mode_on == true {
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

				if *debug_mode_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				// Repeat spot checks
				for time_to_jump_to := crop_start_seconds_int + scan_duration_int ; time_to_jump_to + scan_duration_int < crop_scan_stop_time_int ; time_to_jump_to = time_to_jump_to + spotcheck_interval {

					// Create the ffmpeg command to scan for crop values
					command_to_run_str_slice = nil
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-ss", strconv.Itoa(time_to_jump_to), "-t", scan_duration_str, "-i", inputfile_full_path, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

					if *debug_mode_on == true {
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

				if *debug_mode_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				if *debug_mode_on == true {
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
			if *debug_mode_on == true {

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
			if *subtitle_burn_split == false {
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

		if *subtitle_burn_split == true && subtitle_burn_number > -1 {

			var subtitle_extract_output []string
			var subtitle_extract_error_output []string

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
			if *search_start_str != "" && *fast_search_bool == true {
				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-ss", *search_start_str)
			}

			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-i", inputfile_full_path)

			// The user wants to use the slow and accurate search, place the -ss option after the first -i on ffmpeg commandline.
			if *search_start_str != "" && *fast_search_bool == false {
				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-ss", *search_start_str)
			}

			if *processing_time_str != "" {
				ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-t", *processing_time_str)
			}

			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-vn", "-an", "-filter_complex", "[0:s:" + strconv.Itoa(subtitle_burn_number)+"]copy[subtitle_processing_stream]", "-map", "[subtitle_processing_stream]", filepath.Join(original_subtitles_absolute_path, "subtitle-%10d." + subtitle_stream_image_format))

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Subtitle Extract Options:")
			log_messages_str_slice = append(log_messages_str_slice, "--------------------------------")
			log_messages_str_slice = append(log_messages_str_slice, strings.Join(ffmpeg_subtitle_extract_commandline, " "))

			fmt.Printf("Extracting subtitle stream as %s - images ", subtitle_stream_image_format)

			error_code = nil

			subtitle_extract_output, subtitle_extract_error_output, error_code = run_external_command(ffmpeg_subtitle_extract_commandline)

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
			fmt.Println("took", subtitle_extract_elapsed_time.Round(time.Millisecond))

			//////////////////////////////////////////////////////////////////////////////////////////
			// Process extracted subtitles in as many threads as there are physical processor cores //
			//////////////////////////////////////////////////////////////////////////////////////////

			// Read in subtitle file names
			files_str_slice := read_filenames_in_a_dir(original_subtitles_absolute_path)

			duplicate_removal_start_time := time.Now()
			fmt.Printf("Removing duplicate subtitle slides ")

			v_height := video_height
			v_width := video_width

			if *autocrop_bool == true {
				v_height = strconv.Itoa(crop_values_picture_height)
				v_width = strconv.Itoa(crop_values_picture_width)
			}

			files_remaining := remove_duplicate_subtitle_images (original_subtitles_absolute_path, fixed_subtitles_absolute_path, files_str_slice, v_width, v_height)

			duplicate_removal_elapsed_time := time.Since(duplicate_removal_start_time)
			fmt.Println("took", duplicate_removal_elapsed_time.Round(time.Millisecond))

			subtitle_trimming_start_time := time.Now()

			if *subtitle_burn_resize != "" {

				fmt.Printf("Trimming and resizing subtitle images in multiple threads ")

			} else {

				fmt.Printf("Trimming subtitle images in multiple threads ")
			}

			if *debug_mode_on == true {
				fmt.Println()
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

				go subtitle_trim(original_subtitles_absolute_path, fixed_subtitles_absolute_path, files_remaining[subtitle_start_number : subtitle_end_number], v_width, v_height, process_number, return_channel, *subtitle_burn_resize)

				if *debug_mode_on == true {
					fmt.Println("Process number:", process_number, "started. It processes subtitles:", subtitle_start_number + 1, "-", subtitle_end_number)
				}

				process_number++
				subtitle_start_number =  subtitle_end_number
			}

			// Wait for subtitle processing in goroutines to end
			processes_stopped := 1

			if *debug_mode_on == true {
				fmt.Println()
			}

			for processes_stopped < process_number {
				return_message := <- return_channel

				if *debug_mode_on == true {
					fmt.Println("Process number:", return_message, "ended.")
				}

				processes_stopped++
			}

			subtitle_trimming_elapsed_time := time.Since(subtitle_trimming_start_time)
			fmt.Println("took", subtitle_trimming_elapsed_time.Round(time.Millisecond))

			subtitle_processing_elapsed_time = time.Since(subtitle_processing_start_time)
			fmt.Printf("Complete subtitle processing took %s", subtitle_processing_elapsed_time.Round(time.Millisecond))
			fmt.Println()


			if *debug_mode_on == false {

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

		if *scan_mode_only_bool == false {

			ffmpeg_pass_1_commandline = nil
			ffmpeg_pass_2_commandline = nil

			// Set timecode burn font size
			video_height_int, _ = strconv.Atoi(video_height)
			timecode_font_size = 24

			if *force_hd_bool == true || video_height_int > 700 {
				timecode_font_size = 48
			}

			// Create the start of ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_commandline_start...)

			// If the user wants to use the fast and inaccurate search, place the -ss option before the first -i on ffmpeg commandline.
			if *search_start_str != "" && *fast_search_bool == true {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", *search_start_str)
			}

			// Add possible dvd subtitle color palette hacking option to the FFmpeg commandline.
			// It must be before the first input file to take effect for that file.
			if *subtitle_burn_palette != "" && subtitle_mux_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-palette", *subtitle_burn_palette)
			}

			if split_video == true {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-f", "concat", "-safe", "0", "-i", split_info_file_absolute_path)

			} else if *subtitle_burn_split == true {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", inputfile_full_path, "-f", "image2", "-i", filepath.Join(fixed_subtitles_absolute_path, "subtitle-%10d." + subtitle_stream_image_format))

			} else {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", inputfile_full_path)
			}

			// The user wants to use the slow and accurate search, place the -ss option after the first -i on ffmpeg commandline.
			if *search_start_str != "" && *fast_search_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-ss", *search_start_str)
			}

			if *processing_time_str != "" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-t", *processing_time_str)
			}

			ffmpeg_filter_options := ""
			ffmpeg_filter_options_2 := ""

			/////////////////////////////////////////////////////////////////////////////////////////////////////////
			// If there is no subtitle to process or we are just muxing dvd, dvb or bluray subtitle to target file //
			// then use the simple video processing chain (-vf) in FFmpeg                                          //
			// It has a processing pipeline with only one video input and output                                   //
			/////////////////////////////////////////////////////////////////////////////////////////////////////////

			// Create grayscale FFmpeg - options
			if *grayscale_bool == false {

				grayscale_options = ""

			} else {

				grayscale_options = "lut=u=128:v=128"
			}

			if *burn_timecode_bool == true {
				timecode_burn_options = "drawtext=/usr/share/fonts/TTF/LiberationMono-Regular.ttf:text=%{pts \\\\: hms}:fontcolor=#ffc400:fontsize=" +
					strconv.Itoa(timecode_font_size) + ":box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2:y=(text_h/2)"
			}

			if subtitle_burn_number == -1 || subtitle_mux_bool == true {

				if subtitle_mux_bool == true {
					// There is a dvd, dvb or bluray bitmap subtitle to mux into the target file add the relevant options to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-scodec", "copy")

					for _, subtitle_mux_number := range user_subtitle_mux_numbers_slice {
						ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:s:"+ subtitle_mux_number)
					}

				} else {
					// There is no subtitle to process add the "no subtitle" option to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-sn")
				}
				
				// Add pullup option on the ffmpeg commandline
				if *inverse_telecine == true {
					ffmpeg_filter_options = ffmpeg_filter_options + "pullup"
				}

				// Add deinterlace commands to ffmpeg commandline
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + deinterlace_options

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

				// Add timecode burn in options
				if *burn_timecode_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + timecode_burn_options
				}

				// Add grayscale options to ffmpeg commandline
				if *grayscale_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + grayscale_options
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:v:0", "-vf", ffmpeg_filter_options)

				// Inverse telecine returns frame rate back to original 24 fps
				if *inverse_telecine == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-r", "24")
				}


			} else {
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
				// There is a subtitle to burn into the video, use the complex video processing chain in FFmpeg (-filer_complex) //
				// It can have several simultaneous video inputs and outputs.                                                    //
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

				// Add pullup option on the ffmpeg commandline
				if *inverse_telecine == true {
					ffmpeg_filter_options = ffmpeg_filter_options + "pullup"
				}

				// Add deinterlace commands to ffmpeg commandline
				if ffmpeg_filter_options != "" {
					ffmpeg_filter_options = ffmpeg_filter_options + ","
				}
				ffmpeg_filter_options = ffmpeg_filter_options + deinterlace_options

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

				// Add timecode burn in options
				if *burn_timecode_bool == true {
					ffmpeg_filter_options_2 = ffmpeg_filter_options_2 + "," + timecode_burn_options
				}

				// Add grayscale options to ffmpeg commandline
				if *grayscale_bool == true {
					ffmpeg_filter_options_2 = ffmpeg_filter_options_2 + "," + grayscale_options
				}

				// Add video filter options to ffmpeg commanline
				subtitle_processing_options = "copy"

				// When cropping video widthwise shrink subtitles to fit on top of the cropped video.
				// This results in smaller subtitle font.
				if *autocrop_bool == true && *subtitle_burn_downscale == true {
					subtitle_processing_options = "scale=" + strconv.Itoa(crop_values_picture_width) + ":" + strconv.Itoa(crop_values_picture_height)
				}

				subtitle_source_file := "[0:s:"

				if *subtitle_burn_split == true {

					subtitle_source_file = "[1:v:"
					subtitle_burn_number = 0
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", subtitle_source_file + strconv.Itoa(subtitle_burn_number) +
					"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
					"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
					strconv.Itoa(*subtitle_burn_vertical_offset_int) + ffmpeg_filter_options_2 +
					"[processed_combined_streams]", "-map", "[processed_combined_streams]")

				// Inverse telecine returns frame rate back to original 24 fps
				if *inverse_telecine == true {
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-r", "24")
				}

			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compression options to FFmpeg commandline //
			///////////////////////////////////////////////////////////////////

			// If video vertical resolution is over 700 pixel choose HD video compression settings
			video_compression_options := video_compression_options_sd

			video_height_int, _ = strconv.Atoi(video_height)

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
				audio_compression_options = audio_compression_options_lossless

				// Lossless video compression options
				video_compression_options = video_compression_options_lossless
				video_bitrate = "Lossless"
			}

			number_of_audio_channels_int, _ := strconv.Atoi(number_of_audio_channels)
			bitrate_int := number_of_audio_channels_int * 128
			bitrate_str := strconv.Itoa(bitrate_int) + "k"

			if *audio_compression_aac == true {

				audio_compression_options = nil
				audio_compression_options = []string{"-c:a", "aac", "-b:a", bitrate_str}

			}

			// FIXME When FFmpeg opus support in mp4 is mainlined, remove "-strict", "-2" options from the couple of lines below
			// 2020.11.14: FFmpeg 4.3.1 seems to support opus in mp4 withous strict 2, these can be removed from the following lines
			// If we are encoding audio to opus, then enable FFmpeg experimental features
			// -strict -2 is needed for FFmpeg to use still experimental support for opus in mp4 container.
			if *audio_compression_opus == true {

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

			if *audio_compression_ac3 == true {

				if bitrate_int > 640 {

					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "ac3", "-b:a", "640k"}
				} else {

					audio_compression_options = nil
					audio_compression_options = []string{"-c:a", "ac3", "-b:a", bitrate_str}
				}

			}

			if *no_audio == true {
				audio_compression_options = nil
				audio_compression_options = append(audio_compression_options, "-an")
			}

			// Add video compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, video_compression_options...)

			// Add color subsampling options if needed
			if color_subsampling != "yuv420p" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, color_subsampling_options...)
			}

			// Add audio compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, audio_compression_options...)

			// Add audiomapping options on the commanline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(*audio_stream_number_int))

			ffmpeg_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))

			if *temp_file_directory != "" {
				ffmpeg_2_pass_logfile_path = filepath.Join(*temp_file_directory, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))
			}

			if *fast_encode_bool == false {
				// Add 2 - pass logfile path to ffmpeg commandline
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-passlogfile")
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_2_pass_logfile_path)
			}

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
				fmt.Println("video_compression_options_sd:", video_compression_options_sd)
				fmt.Println("video_compression_options_hd:", video_compression_options_hd)
				fmt.Println("video_compression_options:", video_compression_options)
				fmt.Println("audio_compression_options:", audio_compression_options)
				fmt.Println("denoise_options:", denoise_options)
				fmt.Println("deinterlace_options:", deinterlace_options)
				fmt.Println("ffmpeg_commandline_start:", ffmpeg_commandline_start)
				fmt.Println("subtitle_burn_number:", subtitle_burn_number)
				fmt.Println("subtitle_burn_language_str:", subtitle_burn_language_str)
				fmt.Println("subtitle_burn_vertical_offset_int:", *subtitle_burn_vertical_offset_int)
				fmt.Println("*subtitle_burn_downscale:", *subtitle_burn_downscale)
				fmt.Println("*subtitle_burn_palette:", *subtitle_burn_palette)
				fmt.Println("*subtitle_burn_split:", *subtitle_burn_split)
				fmt.Println("subtitle_mux_bool:", subtitle_mux_bool)
				fmt.Println("user_subtitle_mux_numbers_slice:", user_subtitle_mux_numbers_slice)
				fmt.Println("user_subtitle_mux_languages_slice:", user_subtitle_mux_languages_slice)
				fmt.Println("*grayscale_bool:", *grayscale_bool)
				fmt.Println("grayscale_options:", grayscale_options)
				fmt.Println("color_subsampling_options", color_subsampling_options)
				fmt.Println("*autocrop_bool:", *autocrop_bool)
				fmt.Println("*subtitle_burn_int:", *subtitle_burn_int)
				fmt.Println("*no_deinterlace_bool:", *no_deinterlace_bool)
				fmt.Println("*denoise_bool:", *denoise_bool)
				fmt.Println("*force_hd_bool:", *force_hd_bool)
				fmt.Println("*audio_stream_number_int:", *audio_stream_number_int)
				fmt.Println("*scan_mode_only_bool", *scan_mode_only_bool)
				fmt.Println("*search_start_str", *search_start_str)
				fmt.Println("*processing_stop_time_str", *processing_stop_time_str)
				fmt.Println("*processing_time_str", *processing_time_str)
				fmt.Println("*fast_bool", *fast_bool)
				fmt.Println("*fast_search_bool", *fast_search_bool)
				fmt.Println("*fast_encode_bool", *fast_encode_bool)
				fmt.Println("*burn_timecode_bool", *burn_timecode_bool)
				fmt.Println("timecode_burn_options", timecode_burn_options)
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

				if *inverse_telecine == true {
					fmt.Print("Performing Inverse Telecine on video.\n")
				}

				if frame_rate_str == "29.970" && *inverse_telecine == false {
					fmt.Println("\033[7mWarning: Video frame rate is 29.970. You may need to pullup (Inverse Telecine) this video with option -it\033[0m")
				}

				fmt.Printf("Encoding video with bitrate: %s. ", video_bitrate)

				if color_subsampling != "yuv420p" {
					fmt.Println("Subsampling color:", color_subsampling, "---> yuv420p")
				}

				if *no_audio == true {

					fmt.Println("Audio processing is off.")

				} else if *audio_compression_ac3 == true {

					fmt.Printf("Encoding %s channel audio to ac3 with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])

				} else if *audio_compression_aac == true {

					fmt.Printf("Encoding %s channel audio to aac with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])

				} else if *audio_compression_opus == true {

					fmt.Printf("Encoding %s channel audio to opus with bitrate: %s\n", strconv.Itoa(number_of_audio_channels_int), audio_compression_options[3])
				} else {

					fmt.Printf("Copying %s audio to target.\n", audio_codec)
				}

				fmt.Printf("Pass 1 encoding: " + inputfile_name + " ")
			}

			pass_1_start_time = time.Now()

			ffmpeg_pass_1_output_temp, ffmpeg_pass_1_error_output_temp, error_code := run_external_command(ffmpeg_pass_1_commandline)

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
			fmt.Printf("took %s", pass_1_elapsed_time.Round(time.Millisecond))
			fmt.Println()

			// Add messages to processing log.
			pass_1_commandline_for_logfile := strings.Join(ffmpeg_pass_1_commandline, " ")

			// Make a copy of the FFmpeg commandline for writing in the logfile.
			// Modify commandline so that it works if the user wants to copy and paste it from the logfile and run it.
			// The filter command needs to be in single quotes '
			if subtitle_burn_number == -1 || subtitle_mux_bool == true {

				// Simple processing chain with -vf.
				index := strings.Index(pass_1_commandline_for_logfile, "-vf")
				first_part_of_string := pass_1_commandline_for_logfile[:index + 4]
				first_part_of_string = first_part_of_string + "'"

				second_part_of_string := pass_1_commandline_for_logfile[index + 4:]
				index = strings.Index(second_part_of_string, "-")
				third_part_of_string := second_part_of_string[index - 1:]
				second_part_of_string = second_part_of_string[:index - 1]
				second_part_of_string = second_part_of_string + "'"

				pass_1_commandline_for_logfile = first_part_of_string + second_part_of_string + third_part_of_string

			} else {

				// Complex processing chain with -filter_complex
				index := strings.Index(pass_1_commandline_for_logfile, "-filter_complex")
				first_part_of_string := pass_1_commandline_for_logfile[:index + 16]
				first_part_of_string = first_part_of_string + "'"

				second_part_of_string := pass_1_commandline_for_logfile[index + 16:]
				index = strings.Index(second_part_of_string, "-map")
				third_part_of_string := second_part_of_string[index - 1:]
				second_part_of_string = second_part_of_string[:index - 1]
				second_part_of_string = second_part_of_string + "'"

				pass_1_commandline_for_logfile = first_part_of_string + second_part_of_string + third_part_of_string
			}

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 1 Options:")
			log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
			log_messages_str_slice = append(log_messages_str_slice, pass_1_commandline_for_logfile)

			if *debug_mode_on == true {

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
			if *fast_encode_bool == false {

				if *debug_mode_on == true {

					fmt.Println()
					fmt.Println("ffmpeg_pass_2_commandline:", ffmpeg_pass_2_commandline)

				} else {

					pass_2_elapsed_time = time.Since(start_time)
					fmt.Printf("Pass 2 encoding: " + inputfile_name + " ")
				}

				pass_2_start_time = time.Now()

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

				// Add messages to processing log.
				pass_2_commandline_for_logfile := strings.Join(ffmpeg_pass_2_commandline, " ")

				// Make a copy of the FFmpeg commandline for writing in the logfile.
				// Modify commandline so that it works if the user wants to copy and paste it from the logfile and run it.
				if subtitle_burn_number == -1 || subtitle_mux_bool == true {
					// Simple processing chain with -vf.
					pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, "-vf ", "-vf '", 1)
					pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, " -c:v", "' -c:v", 1)
				} else {
					// Complex processing chain with -filter_complex
					pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, "-filter_complex ", "-filter_complex '", 1)
					pass_2_commandline_for_logfile = strings.Replace(pass_2_commandline_for_logfile, "[processed_combined_streams] -map", "[processed_combined_streams]' -map", 1)
				}

				log_messages_str_slice = append(log_messages_str_slice, "")
				log_messages_str_slice = append(log_messages_str_slice, "FFmpeg Pass 2 Options:")
				log_messages_str_slice = append(log_messages_str_slice, "-----------------------")
				log_messages_str_slice = append(log_messages_str_slice, pass_2_commandline_for_logfile)

				if *debug_mode_on == true {

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

			////////////////////////////
			// Remove temporary files //
			////////////////////////////

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log")
			}

			if _, err := os.Stat(ffmpeg_2_pass_logfile_path + "-0.log.mbtree"); err == nil {
				os.Remove(ffmpeg_2_pass_logfile_path + "-0.log.mbtree")
			}

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

			if *subtitle_burn_split == true {
				if *debug_mode_on == true {
					fmt.Println("\nExtracted subtitle images are not deleted in debug - mode.\n")
				} else {

					// Remove subtitle directories.
					if _, err := os.Stat(original_subtitles_absolute_path); err == nil {
						os.RemoveAll(original_subtitles_absolute_path)
					}

					if _, err := os.Stat(fixed_subtitles_absolute_path); err == nil {
						os.RemoveAll(fixed_subtitles_absolute_path)
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

