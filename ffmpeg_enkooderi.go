package main

import (
	"bytes"
	"flag"
	"fmt"
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
var version_number string = "1.43" // This is the version of this program
var Complete_stream_info_map = make(map[int][]string)
var video_stream_info_map = make(map[string]string)
var audio_stream_info_map = make(map[string]string)
var subtitle_stream_info_map = make(map[string]string)
var wrapper_info_map = make(map[string]string)

// Create a slice for storing all video, audio and subtitle stream infos for each input file.
// There can be many audio and subtitle streams in a file.
var Complete_file_info_slice [][][][]string

func run_external_command(command_to_run_str_slice []string) (stdout_output []string, stderr_output string, error_code error) {

	command_output_str := ""

	// Create the struct needed for running the external command
	command_struct := exec.Command(command_to_run_str_slice[0], command_to_run_str_slice[1:]...)

	// Run external command
	var stdout, stderr bytes.Buffer
	command_struct.Stdout = &stdout
	command_struct.Stderr = &stderr

	error_code = command_struct.Run()

	command_output_str = string(stdout.Bytes())
	stderr_output = string(stderr.Bytes())

	// Split the output of the command to lines and store in a slice
	for _, line := range strings.Split(command_output_str, "\n") {
		stdout_output = append(stdout_output, line)
	}

	return stdout_output, stderr_output, error_code
}

func find_executable_path(filename string) (file_path string) {

	/////////////////////////////////////////////////
	// Test if executable can be found in the path //
	/////////////////////////////////////////////////

	if _, error := exec.LookPath("ffmpeg"); error != nil {
		fmt.Println()
		fmt.Println("Error, cant find FFmpeg in path, can't continue.")
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

			// Add also duration from wrapper information to the video info.
			single_video_stream_info_slice = append(single_video_stream_info_slice, file_name, video_stream_info_map["width"], video_stream_info_map["height"], wrapper_info_map["duration"], video_stream_info_map["codec_name"], video_stream_info_map["pix_fmt"], video_stream_info_map["color_space"])
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

func convert_timecode_to_seconds(timestring string) (string, string) {
	var hours_int, minutes_int, seconds_int, seconds_total_int int
	var hours_str, minutes_str, seconds_str, milliseconds_str string
	var seconds_total_str, error_happened string

	if strings.ContainsAny(timestring, ".") {
		milliseconds_str = strings.Split(timestring, ".")[1]
		timestring = strings.Replace(timestring, "."+milliseconds_str, "", 1)

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

func convert_seconds_to_timecode(cut_positions_after_processing_seconds []string) []string {
	var cut_positions_as_timecodes []string

	for counter, item := range cut_positions_after_processing_seconds {

		// Remove the first edit point if it is zero, as this really is no edit point
		if counter == 0 && item == "0" {
			continue
		}

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

		cut_positions_as_timecodes = append(cut_positions_as_timecodes, timecode)
	}

	return cut_positions_as_timecodes
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
			temp_2_str_slice := convert_seconds_to_timecode(temp_str_slice)

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
	cut_positions_as_timecodes = convert_seconds_to_timecode(cut_positions_after_processing_seconds)

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

//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

func main() {

	//////////////////////////////////////////
	// Define and parse commandline options //
	//////////////////////////////////////////
	// Audio options
	var audio_language_str = flag.String("a", "", "Audio language: -a fin or -a eng or -a ita  Only use option -an or -a not both.")
	var audio_stream_number_int = flag.Int("an", 0, "Audio stream number, -a 1 (Use audio stream number 1 from the source file).")
	var audio_compression_ac3 = flag.Bool("ac3", false, "Compress audio as ac3. Channel count adjusts compression bitrate automatically. Stereo uses 256k and 3 - 6 channels uses 640k bitrate.")

	// Video options
	var autocrop_bool = flag.Bool("ac", false, "Autocrop. Find crop values automatically by doing 10 second spot checks in 10 places for the duration of the file.")
	var denoise_bool = flag.Bool("dn", false, "Denoise. Use HQDN3D - filter to remove noise from the picture. This option is equal to Hanbrakes 'medium' noise reduction settings.")
	var grayscale_bool = flag.Bool("gr", false, "Convert video to Grayscale. Use this option if the original source is black and white. This results more bitrate being available for b/w information and better picture quality.")
	var force_hd_bool = flag.Bool("hd", false, "Force video encoding to use HD bitrate and profile (Profile = High, Level = 4.1, Bitrate = 8000k) By default this program decides video encoding profile and bitrate automatically depending on the vertical resolution of the picture.")
	var no_deinterlace_bool = flag.Bool("nd", false, "No Deinterlace. By default deinterlace is always used. This option disables it.")
	var split_times = flag.String("st", "", "Split out parts of the file. Give colon separated start and stop times for the parts of the file to use, for example: -st 0,10:00,01:35:12.800,01:52:14 defines that 0 secs - 10 mins of the start of the file will be used and joined to the next part that starts at 01 hours 35 mins 12 seconds and 800 milliseconds and stops at 01 hours 52 mins 14 seconds. Don't use space - characters. A zero or word 'start' can be used to mean the absolute start of the file and word 'end' the end of the file. Both start and stop times must be defined.")
	var burn_timecode_bool = flag.Bool("tc", false, "Burn timecode on top of the video. Timecode can be used to look for exact edit points for the file split feature")

	// Options that affect both video and audio
	var force_lossless_bool = flag.Bool("ls", false, "Force encoding to use lossless 'utvideo' compression for video and 'flac' compression for audio. This also turns on -fe")

	// Subtitle options
	var subtitle_language_str = flag.String("s", "", "Subtitle language: -s fin or -s eng -s ita  Only use option -sn or -s not both.")
	var subtitle_downscale = flag.Bool("sd", false, "Subtitle `downscale`. When cropping video widthwise, scale down subtitle to fit on top of the cropped video instead of cropping the subtitle. This option results in smaller subtitle font.")
	var subtitle_int = flag.Int("sn", -1, "Subtitle stream `number, -sn 1` Use subtitle number 1 from the source file. Only use option -sn or -s not both.")
	var subtitle_vertical_offset_int = flag.Int("so", 0, "Subtitle `offset`, -so 55 (move subtitle 55 pixels down), -so -55 (move subtitle 55 pixels up).")
	var subtitle_mux_bool = flag.Bool("sm", false, "Mux subtitle into the target file. This only works with dvd, dvb and bluray bitmap based subtitles. If this option is not set then subtitles will be burned into the video. This option can not be used by itself, it must be used with -s or -sn. mp4 only supports DVD and DVB subtitles not Bluray. Bluray subtitles can be muxed into an mkv file.")
	var subtitle_palette = flag.String("palette", "", "Hack dvd subtitle color palette. Option takes 1-16 comma separated hex numbers ranging from 0 to f. Zero = black, f = white, so only shades between black -> gray -> white can be defined. FFmpeg requires 16 hex numbers, so f's are automatically appended to the end of user given numbers. Each dvd uses color mapping differently so you need to try which numbers control the colors you want to change. Usually the first 4 numbers control the colors. Example: -palette f,0,f")
	var subtitle_split = flag.Bool("sp", false, "Subtitle Split. Move subtitles that are above the center of the screen up to the top of the screen and subtitles below center down on the bottom of the screen")

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
	var show_program_version_short = flag.Bool("v", false, "Show the version of this program.")
	var show_program_version_long = flag.Bool("version", false, "Show the version of this program.")

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
	var error_code error
	var error_messages []string
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

	start_time := time.Now()
	file_split_start_time := time.Now()
	file_split_elapsed_time := time.Since(file_split_start_time)
	pass_1_start_time := time.Now()
	pass_1_elapsed_time := time.Since(pass_1_start_time)
	pass_2_start_time := time.Now()
	pass_2_elapsed_time := time.Since(pass_2_start_time)
	subtitle_extract_start_time := time.Now()
	subtitle_extract_elapsed_time := time.Since(subtitle_extract_start_time)

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

	/////////////////////////////////////////////////////////
	// Test if needed executables can be found in the path //
	/////////////////////////////////////////////////////////
	find_executable_path("ffmpeg")
	find_executable_path("ffprobe")

	if *subtitle_split == true {
		find_executable_path("convert")
		find_executable_path("mogrify")
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

	// Disable -ss and -t options if user did use the -st option and input some edit times
	if split_video == true {
		*search_start_str = ""
		*processing_time_str = ""
	}

	// Always use 1-pass encoding with lossless encoding. Turn on option -fe.
	if *force_lossless_bool == true {
		*fast_encode_bool = true
	}

	// Check dvd palette hacking option string correctness.
	if *subtitle_palette != "" {
		temp_slice := strings.Split(*subtitle_palette, ",")
		*subtitle_palette = ""
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

			*subtitle_palette = *subtitle_palette + strings.Repeat(strings.ToLower(character), 6)

			if counter < len(temp_slice)-1 {
				*subtitle_palette = *subtitle_palette + ","
			}

		}

		if len(temp_slice) < 16 {

			*subtitle_palette = *subtitle_palette + ","

			for counter := len(temp_slice); counter < 16; counter++ {
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
	audio_compression_options_2_channels_ac3 := []string{"-c:a", "ac3", "-b:a", "256k"}
	audio_compression_options_6_channels_ac3 := []string{"-c:a", "ac3", "-b:a", "640k"}
	audio_compression_options_lossless_flac := []string{"-acodec", "flac"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}
	color_subsampling_options := []string{"-pix_fmt", "yuv420p"}
	var ffmpeg_commandline_start []string

	//////////////////////////
	// Imagemagick Options //
	//////////////////////////
	picture_margin := 10

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

	if *debug_mode_on == true {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-hide_banner", "-threads", "auto")
	} else {
		ffmpeg_commandline_start = append(ffmpeg_commandline_start, "ffmpeg", "-y", "-loglevel", "8", "-threads", "auto")
	}

	subtitle_number := *subtitle_int

	///////////////////////////////
	// Scan inputfile properties //
	///////////////////////////////

	for _, file_name := range input_filenames {

		// Get video info with: ffprobe -loglevel 16 -show_entries format:stream -print_format flat -i InputFile
		command_to_run_str_slice = nil
		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe", "-loglevel", "8", "-show_entries", "format:stream", "-print_format", "flat", "-i")

		if *debug_mode_on == true {
			fmt.Println()
			fmt.Println("command_to_run_str_slice:", command_to_run_str_slice, file_name)
		}

		command_to_run_str_slice = append(command_to_run_str_slice, file_name)

		unsorted_ffprobe_information_str_slice, _, error_code = run_external_command(command_to_run_str_slice)

		if error_code != nil {

			fmt.Println("\n\nFFprobe reported error:", unsorted_ffprobe_information_str_slice, "\n")
			os.Exit(1)
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

	for _, file_info_slice := range Complete_file_info_slice {

		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]

		audio_slice := file_info_slice[1]
		subtitle_slice := file_info_slice[2]

		file_name_temp := video_slice[0]
		file_name := filepath.Base(file_name_temp)
		video_width := video_slice[1]
		video_height := video_slice[2]

		if video_width == "0" || video_height == "0" {
			error_messages = append(error_messages, "File: '"+file_name+"' does not have a video stream.")
		}

		// If user gave audio stream number, check that we have at least that much audio streams in the source file.
		if len(audio_slice)-1 < *audio_stream_number_int {
			error_messages = append(error_messages, "File: '"+file_name+"' does not have an audio stream number: "+strconv.Itoa(*audio_stream_number_int))
		}

		// If user gave subtitle stream number, check that we have at least that much subtitle streams in the source file.
		if len(subtitle_slice)-1 < subtitle_number {
			error_messages = append(error_messages, "File: '"+file_name+"' does not have an subtitle stream number: "+strconv.Itoa(subtitle_number))
		}
	}

	// If there were error messages then we can't process all files that the user gave on the commandline, inform the user and exit.
	if len(error_messages) > 0 {

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

		for _, file_info_slice := range Complete_file_info_slice {
			video_slice_temp := file_info_slice[0]
			video_slice := video_slice_temp[0]
			audio_slice := file_info_slice[1]
			subtitle_slice := file_info_slice[2]

			file_to_process_temp := video_slice[0]
			file_to_process = filepath.Base(file_to_process_temp)
			video_width = video_slice[1]
			video_height = video_slice[2]
			video_codec_name = video_slice[4]
			color_subsampling = video_slice[5]
			color_space = video_slice[6]

			fmt.Println()
			subtitle_text := "File name '" + file_to_process + "'"
			text_length := len(subtitle_text)
			fmt.Println(subtitle_text)
			fmt.Println(strings.Repeat("-", text_length))

			fmt.Println("Video width:", video_width, ", height:", video_height, ", codec:", video_codec_name, ", color subsampling:", color_subsampling, ", color space:", color_space)
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

		for _, file_info_slice := range Complete_file_info_slice {

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

		for _, file_info_slice := range Complete_file_info_slice {

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

	for _, file_info_slice := range Complete_file_info_slice {

		subtitle_horizontal_offset_int = 0
		subtitle_horizontal_offset_str = "0"
		start_time = time.Now()
		video_slice_temp := file_info_slice[0]
		video_slice := video_slice_temp[0]
		file_name := video_slice[0]
		video_width = video_slice[1]
		video_height = video_slice[2]
		video_duration = video_slice[3]
		video_codec_name = video_slice[4]
		color_subsampling = video_slice[5]
		color_space = video_slice[6]

		// Create input + output filenames and paths
		inputfile_absolute_path, _ := filepath.Abs(file_name)
		inputfile_path := filepath.Dir(inputfile_absolute_path)
		inputfile_name := filepath.Base(file_name)
		input_filename_extension := filepath.Ext(inputfile_name)
		output_file_absolute_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension)+output_filename_extension)
		subtitles_absolute_extract_path := filepath.Join(inputfile_path, output_directory_name, subtitle_extract_dir, inputfile_name + "-"+original_subtitles_dir)
		fixed_subtitles_absolute_path := filepath.Join(inputfile_path, output_directory_name, subtitle_extract_dir, inputfile_name + "-" + fixed_subtitles_dir)

		if *debug_mode_on == true {
			fmt.Println("inputfile_path:", inputfile_path)
			fmt.Println("inputfile_name:", inputfile_name)
			fmt.Println("output_file_absolute_path:", output_file_absolute_path)
			fmt.Println("video_width:", video_width)
			fmt.Println("video_height:", video_height)
			fmt.Println("orig_subtitle_path", orig_subtitle_path)
			fmt.Println("cropped_subtitle_path", cropped_subtitle_path)
		}

		// Add messages to processing log.
		var log_messages_str_slice []string
		log_messages_str_slice = append(log_messages_str_slice, "")
		log_messages_str_slice = append(log_messages_str_slice, "Filename: "+file_name)
		underline_length := len(file_name) + len("Filename: ") + 1
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

		/////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Find stream audio number corresponding to the audio language name (eng, fin, ita) user possibly gave us //
		/////////////////////////////////////////////////////////////////////////////////////////////////////////////
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

		if split_video == true {
			audio_codec = "flac"
		}

		if *use_matroska_container == false {

			if audio_codec != "aac" && audio_codec != "ac3" && audio_codec != "mp2" && audio_codec != "mp3" && audio_codec != "dts" {
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

		///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Find subtitle stream number corresponding to the subtitle language name (eng, fin, ita) user possibly gave us //
		///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
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

		////////////////////////////////////////////////////
		// Split out and use only some parts of the video //
		////////////////////////////////////////////////////
		if split_video == true {

			file_split_start_time = time.Now()
			counter_2 := 0

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

			for counter := 0; counter < len(cut_list_seconds_str_slice); counter = counter + 2 {
				counter_2++
				splitfile_name := "splitfile-" + strconv.Itoa(counter_2) + output_matroska_filename_extension
				split_file_path := filepath.Join(inputfile_path, output_directory_name, splitfile_name)
				list_of_splitfiles = append(list_of_splitfiles, split_file_path)

				ffmpeg_file_split_commandline = nil
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, ffmpeg_commandline_start...)
				ffmpeg_file_split_commandline = append(ffmpeg_file_split_commandline, "-i", file_name, "-ss", cut_list_seconds_str_slice[counter])

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

				file_split_output_temp, _, error_code := run_external_command(ffmpeg_file_split_commandline)

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", file_split_output_temp, "\n")
					os.Exit(1)
				}
			}

			file_split_elapsed_time = time.Since(file_split_start_time)
			fmt.Printf("\nSplitfile creation took %s\n", file_split_elapsed_time.Round(time.Millisecond))
			fmt.Println()

			log_messages_str_slice = append(log_messages_str_slice, "\nSplitfile creation took "+file_split_elapsed_time.Round(time.Millisecond).String())

		}

		////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
		// Subtitle Split. Move subtitles that are above the center of the screen up to the top of the screen and subtitles below center down on the bottom of the screen //
		////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

		if *subtitle_split == true && subtitle_number != -1 {

			var subtitle_extract_output []string

			subtitle_extract_start_time = time.Now()

			// Create output subdirectories
			if _, err := os.Stat(subtitles_absolute_extract_path); os.IsNotExist(err) {
				os.MkdirAll(subtitles_absolute_extract_path, 0777)
			}

			if _, err := os.Stat(fixed_subtitles_absolute_path); os.IsNotExist(err) {
				os.MkdirAll(fixed_subtitles_absolute_path, 0777)
			}

			// Extract subtitle stream as on tiff for every frame of the movie.
			ffmpeg_subtitle_extract_commandline = nil
			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, ffmpeg_commandline_start...)
			ffmpeg_subtitle_extract_commandline = append(ffmpeg_subtitle_extract_commandline, "-i", file_name, "-vn", "-an", "-filter_complex", "[0:s:" + strconv.Itoa(subtitle_number)+"]copy[subtitle_processing_stream]", "-map", "[subtitle_processing_stream]", filepath.Join(subtitles_absolute_extract_path, "subtitle-%10d.tiff"))

			fmt.Println("Extracting subtitle stream as tiff - images. This might take a long time ...")

			error_code = nil
			subtitle_extract_output, _, error_code = run_external_command(ffmpeg_subtitle_extract_commandline)

			if error_code != nil {

				fmt.Println("\n\nFFmpeg reported error:", subtitle_extract_output, "\n")
				os.Exit(1)
			}

			if len(subtitle_extract_output) != 0 && strings.TrimSpace(subtitle_extract_output[0]) != "" {
				fmt.Println("\n", subtitle_extract_output, "\n")
			}

			subtitle_extract_elapsed_time = time.Since(subtitle_extract_start_time)
			fmt.Printf("\nSubtitle extract processing took %s\n", subtitle_extract_elapsed_time.Round(time.Millisecond))
			fmt.Println()

			// Trimm extracted subtitles
			fmt.Println("Trimming subtitle images. This might take a long time ...")

			// Read in subtitle file names
			files_str_slice := read_filenames_in_a_dir(subtitles_absolute_extract_path)
			number_of_subtitle_files := len(files_str_slice)

			var subtitles_dimension_map = make(map[string][]string)
			var subtitle_dimension_info []string

			for subtitle_counter, subtitle_name := range files_str_slice {

				fmt.Printf("\rProcessing image %d / %d", subtitle_counter+1, number_of_subtitle_files)

				subtitle_trim_output, subtitle_trim_error, _ := run_external_command([]string{"convert", "-trim", "-print", "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]", filepath.Join(subtitles_absolute_extract_path, subtitle_name), filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})

				// If there is no subtitle in the image, then copy over the original image.
				if subtitle_trim_error != "" {

					// Copy original empty image over

					// FIXME
					// _, subtitle_trim_error, error_code := run_external_command([]string{"convert", filepath.Join(subtitles_absolute_extract_path, subtitle_name), filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})
					_, subtitle_trim_error, error_code := run_external_command([]string{"convert", "-size", video_width + "x" + video_height, "canvas:transparent", "-alpha", "on", filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})

					// FIXME
					// _, subtitle_trim_error, error_code := run_external_command([]string{"cp", "-f", filepath.Join(subtitles_absolute_extract_path, subtitle_name), filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})

					if error_code != nil {
						fmt.Println("ImageMagick convert reported error:", subtitle_trim_error)
					}
					continue
				}

				// Take image properties before and after crop and store them in a map.
				// Image info n 'subtitle_dimension_info' is:
				// Original width
				// Original height
				// Cropped width
				// Cropped height
				// Start of crop on x axis
				// Start of crop on y axis

				subtitle_dimension_info = strings.Split(subtitle_trim_output[0], ",")
				subtitles_dimension_map[subtitle_name] = subtitle_dimension_info

			}

			fmt.Println()

			// Overlay cropped subtitles on a new position on a transparent canvas
			fmt.Println("\nAdjusting subtitle position on picture ...")

			var subtitle_new_y int
			number_of_keys := len(subtitles_dimension_map)
			counter := 0

			for subtitle_name := range subtitles_dimension_map {

				counter++

				fmt.Printf("\rProcessing image %d / %d", counter, number_of_keys)

				// orig_width ,_ := strconv.Atoi(subtitles_dimension_map[subtitle_name][0])
				// orig_height ,_:= strconv.Atoi(subtitles_dimension_map[subtitle_name][1])
				cropped_width, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][2])
				cropped_height, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][3])
				// cropped_start_x ,_:= strconv.Atoi(subtitles_dimension_map[subtitle_name][4])
				cropped_start_y, _ := strconv.Atoi(subtitles_dimension_map[subtitle_name][5])

				video_height_int, _ := strconv.Atoi(video_height)
				video_width_int, _ := strconv.Atoi(video_width)
				picture_center := video_height_int / 2
				subtitle_new_x := (video_width_int / 2) - (cropped_width / 2) // This centers cropped subtitle on the x axis

				if cropped_start_y > picture_center {

					// Center subtitle on the bottom of the picure
					subtitle_new_y = video_height_int - cropped_height - picture_margin

				} else {

					// Center subtitle on top of the picure
					subtitle_new_y = picture_margin
				}

				// FIXME
				// _, subtitle_trim_error, error_code := run_external_command([]string{"convert", "-colorspace", "gray", "-size", strconv.Itoa(orig_width) + "x" + strconv.Itoa(orig_height), "canvas:transparent", filepath.Join(fixed_subtitles_absolute_path, subtitle_name), "-geometry", "+" + strconv.Itoa(subtitle_new_x) + "+" + strconv.Itoa(subtitle_new_y), "-composite", "-compose", "over", filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})
				_, subtitle_trim_error, error_code := run_external_command([]string{"convert", "-size", video_width + "x" + video_height, "canvas:transparent", filepath.Join(fixed_subtitles_absolute_path, subtitle_name), "-geometry", "+" + strconv.Itoa(subtitle_new_x) + "+" + strconv.Itoa(subtitle_new_y), "-composite", "-compose", "over", filepath.Join(fixed_subtitles_absolute_path, subtitle_name)})

				if error_code != nil {
					fmt.Println("ImageMagick convert reported error:", subtitle_trim_error)
				}

			}

			fmt.Println()
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

			video_duration_int, _ := strconv.Atoi(strings.Split(video_duration, ".")[0])

			// For long videos take short snapshots of crop values spanning the whole file. This is "quick scan mode".
			if video_duration_int > 300 {

				spotcheck_interval := video_duration_int / 10 // How many spot checks will be made across the duration of the video (default = 10)
				scan_duration_str := "10"                     // How many seconds of video to scan for each spot (default = 10 seconds)
				scan_duration_int, _ := strconv.Atoi(scan_duration_str)

				if *debug_mode_on == false {
					fmt.Printf("Finding crop values for: " + inputfile_name + "   ")
				}

				// Repeat spot checks
				for time_to_jump_to := scan_duration_int; time_to_jump_to+scan_duration_int < video_duration_int; time_to_jump_to = time_to_jump_to + spotcheck_interval {

					// Create the ffmpeg command to scan for crop values
					command_to_run_str_slice = nil
					command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg", "-ss", strconv.Itoa(time_to_jump_to), "-t", scan_duration_str, "-i", file_name, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:8:250", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

					if *debug_mode_on == true {
						fmt.Println()
						fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
						fmt.Println()
					}

					ffmpeg_crop_output, _, error_code := run_external_command(command_to_run_str_slice)

					if error_code != nil {

						fmt.Println("\n\nFFmpeg reported error:", ffmpeg_crop_output, "\n")
						os.Exit(1)
					}

					// Parse the crop value list to find the value that is most frequent, that is the value that can be applied without cropping too much or too little.
					if error_code == nil {

						crop_value_counter := 0

						for _, slice_item := range ffmpeg_crop_output {

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

				ffmpeg_crop_output, _, error_code := run_external_command(command_to_run_str_slice)

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", ffmpeg_crop_output, "\n")
					os.Exit(1)
				}

				// FFmpeg collects possible crop values across the first 1800 seconds of the file and outputs a list of how many times each possible crop values exists.
				// Parse the list to find the value that is most frequent, that is the value that can be applied without cropping too musch or too little.
				if error_code == nil {

					crop_value_counter := 0

					for _, slice_item := range ffmpeg_crop_output {

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
					fmt.Println(ffmpeg_crop_output)
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
			subtitle_horizontal_offset_int = crop_values_width_offset * -1
			subtitle_horizontal_offset_str = strconv.Itoa(subtitle_horizontal_offset_int)

			fmt.Println("Top:", crop_values_height_offset, ", Bottom:", strconv.Itoa(cropped_height), ", Left:", crop_values_width_offset, ", Right:", strconv.Itoa(cropped_width))

			log_messages_str_slice = append(log_messages_str_slice, "")
			log_messages_str_slice = append(log_messages_str_slice, "Crop values are, Top: "+strconv.Itoa(crop_values_height_offset)+", Bottom: "+strconv.Itoa(cropped_height)+", Left: "+strconv.Itoa(crop_values_width_offset)+", Right: "+strconv.Itoa(cropped_width))
			log_messages_str_slice = append(log_messages_str_slice, "After cropping video width is: "+strconv.Itoa(crop_values_picture_width)+", and height is: "+strconv.Itoa(crop_values_picture_height))

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
			if *subtitle_palette != "" && *subtitle_mux_bool == false {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-palette", *subtitle_palette)
			}

			if split_video == true {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-f", "concat", "-safe", "0", "-i", split_info_file_absolute_path)

			} else if *subtitle_split == true {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", file_name, "-f", "image2", "-i", filepath.Join(fixed_subtitles_absolute_path, "subtitle-%10d.tiff"))

			} else {

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", file_name)
			}

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

			// Create grayscale FFmpeg - options
			if *grayscale_bool == false {

				grayscale_options = ""

			} else {

				if subtitle_number == -1 {
					grayscale_options = "lut=u=128:v=128"
				}

				if subtitle_number >= 0 {
					grayscale_options = ",lut=u=128:v=128"
				}
			}

			if *burn_timecode_bool == true {
				timecode_burn_options = ",drawtext=/usr/share/fonts/TTF/LiberationMono-Regular.ttf:text=%{pts \\\\: hms}:fontcolor=#ffc400:fontsize=" +
					strconv.Itoa(timecode_font_size) + ":box=1:boxcolor=black@0.7:boxborderw=10:x=(w-text_w)/2:y=(text_h/2)"
			}

			if subtitle_number == -1 || *subtitle_mux_bool == true {

				if *subtitle_mux_bool == true {
					// There is a dvd, dvb or bluray bitmap subtitle to mux into the target file add the relevant options to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-scodec", "copy", "-map", "0:s:"+strconv.Itoa(subtitle_number))
				} else {
					// There is no subtitle to process add the "no subtitle" option to FFmpeg commandline.
					ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-sn")
				}

				// Add deinterlace commands to ffmpeg commandline
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
					ffmpeg_filter_options = ffmpeg_filter_options + timecode_burn_options[1:]
				}

				// Add grayscale options to ffmpeg commandline
				if *grayscale_bool == true {
					if ffmpeg_filter_options != "" {
						ffmpeg_filter_options = ffmpeg_filter_options + ","
					}
					ffmpeg_filter_options = ffmpeg_filter_options + grayscale_options
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:v:0", "-vf", ffmpeg_filter_options)

			} else {
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////
				// There is a subtitle to burn into the video, use the complex video processing chain in FFmpeg (-filer_complex) //
				// It can have several simultaneous video inputs and outputs.                                                    //
				///////////////////////////////////////////////////////////////////////////////////////////////////////////////////

				// Add deinterlace commands to ffmpeg commandline
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

				// Add video filter options to ffmpeg commanline
				subtitle_processing_options = "copy"

				// When cropping video widthwise shrink it to fit on top of the cropped video.
				// This results in smaller subtitle font.
				if *autocrop_bool == true && *subtitle_downscale == true {
					subtitle_processing_options = "scale=" + strconv.Itoa(crop_values_picture_width) + ":" + strconv.Itoa(crop_values_picture_height)
				}

				subtitle_source_file := "[0:s:"

				if *subtitle_split == true {

					subtitle_source_file = "[1:v:"
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", subtitle_source_file + strconv.Itoa(subtitle_number) +
					"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
					"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=" + subtitle_horizontal_offset_str + ":main_h-overlay_h+" +
					strconv.Itoa(*subtitle_vertical_offset_int) + timecode_burn_options+grayscale_options +
					"[processed_combined_streams]", "-map", "[processed_combined_streams]")
			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compressing options to FFmpeg commandline //
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
				audio_compression_options = audio_compression_options_lossless_flac

				// Lossless video compression options
				video_compression_options = video_compression_options_lossless
				video_bitrate = "Lossless"
			}

			if *audio_compression_ac3 == true {

				number_of_audio_channels_int, _ := strconv.Atoi(number_of_audio_channels)

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

			// Add color subsampling options if needed
			if color_subsampling != "yuv420p" {
				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, color_subsampling_options...)
			}

			// Add audio compression options to ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, audio_compression_options...)

			// Add audiomapping options on the commanline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-map", "0:a:" + strconv.Itoa(*audio_stream_number_int))

			ffmpeg_2_pass_logfile_path := filepath.Join(inputfile_path, output_directory_name, strings.TrimSuffix(inputfile_name, input_filename_extension))

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
				fmt.Println("subtitle_number:", subtitle_number)
				fmt.Println("subtitle_language_str:", subtitle_language_str)
				fmt.Println("subtitle_vertical_offset_int:", *subtitle_vertical_offset_int)
				fmt.Println("*subtitle_downscale:", *subtitle_downscale)
				fmt.Println("*subtitle_palette:", *subtitle_palette)
				fmt.Println("*subtitle_mux_bool:", *subtitle_mux_bool)
				fmt.Println("*subtitle_split:", *subtitle_split)
				fmt.Println("*grayscale_bool:", *grayscale_bool)
				fmt.Println("grayscale_options:", grayscale_options)
				fmt.Println("color_subsampling_options", color_subsampling_options)
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
				fmt.Println("Encoding with video bitrate:", video_bitrate)

				if color_subsampling != "yuv420p" {
					fmt.Println("Subsampling color:", color_subsampling, "---> yuv420p")
				}

				if *audio_compression_ac3 == true {

					fmt.Println("Encoding audio to ac3 with bitrate:", audio_compression_options[3])

				} else {

					fmt.Printf("Copying %s audio to target.\n", audio_codec)
				}

				fmt.Printf("Pass 1 encoding: " + inputfile_name + " ")
			}

			pass_1_start_time = time.Now()

			ffmpeg_pass_1_output_temp, _, error_code := run_external_command(ffmpeg_pass_1_commandline)

			if error_code != nil {

				fmt.Println("\n\nFFmpeg reported error:", ffmpeg_pass_1_output_temp, "\n")
				os.Exit(1)
			}

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

				ffmpeg_pass_2_output_temp, _, error_code := run_external_command(ffmpeg_pass_2_commandline)

				if error_code != nil {

					fmt.Println("\n\nFFmpeg reported error:", ffmpeg_pass_2_output_temp, "\n")
					os.Exit(1)
				}

				pass_2_elapsed_time = time.Since(pass_2_start_time)
				fmt.Printf("took %s", pass_2_elapsed_time.Round(time.Millisecond))
				fmt.Println()

				if split_video == true {
					fmt.Println("\nPlease check the following edit positions for possible video / audio glitches and adjust split times if needed: ")

					for _, timecode := range cut_positions_as_timecodes {
						fmt.Println(timecode)
					}
				}

				fmt.Println()

				// Add messages to processing log.
				pass_2_commandline_for_logfile := strings.Join(ffmpeg_pass_2_commandline, " ")

				// Make a copy of the FFmpeg commandline for writing in the logfile.
				// Modify commandline so that it works if the user wants to copy and paste it from the logfile and run it.
				if subtitle_number == -1 || *subtitle_mux_bool == true {
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

			elapsed_time := time.Since(start_time)
			fmt.Printf("Processing took %s", elapsed_time.Round(time.Millisecond))
			fmt.Println()

			// Add messages to processing log.
			log_messages_str_slice = append(log_messages_str_slice, "")
			pass_1_elapsed_time := pass_1_elapsed_time.Round(time.Millisecond)
			pass_2_elapsed_time := pass_2_elapsed_time.Round(time.Millisecond)
			total_elapsed_time := elapsed_time.Round(time.Millisecond)
			log_messages_str_slice = append(log_messages_str_slice, "Pass 1 took: "+pass_1_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "Pass 2 took: "+pass_2_elapsed_time.String())
			log_messages_str_slice = append(log_messages_str_slice, "Processing took: "+total_elapsed_time.String())

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

//
// // FIXME
//
// Lue -sp koodi vielä ajatuksella läpi, sinne on saattanut jäädä epäloogisuuksia ja sotkuja.
// -sp toimii tiff formaatilla nyt dvd:n osalta, mutta nykii edelleen BluRayssä (RedDwarf). Tää on ehkä nyt korjattu.
// -sp marginaali näytön ylä- tai alareunaa pitää skaalautua videon reson mukaa. BluRayssä 10 pixeliä on aika hyvä dvdssä sen pitäis olla pienempi.
// Mitä taphtuu kun -sp ja -autocrop on päällä, mitä subtitleille tapahtuu ?
// -sp subtitleprosessointia pitää saada nopeutettua: tunnista tyhjät slaidit ja tallenna vain yksi tyjä ja linkkaa muut siihen. Tunnista samat subtitlet ja tallenna vain yksi ja linkkaa muut siihen. Jos tämä ei toimi tee subtitlekäsittely samanaikaisesti säikeissä.
// Tutki tukeeko ffmpeg jotain subtitleformaattia, jossa aikakoodit on tekstitiedostossa ja subtitlet kuvina. Jos tukee tee tuki tälle formaatille ja tallenna -sp - optiolla subtitlet kyseiseen formaattiin.
//
//
// ffmpeg -y -i movie.mov -loop 1 -i overlay.png -loop 1 -i fademe.png \ -filter_complex '[0:v][1:v] overlay [V1]; \ [2:v] fade=out:25:25:alpha=1 [V2]; [V1][V2] overlay' \ faded.mp4
// ffmpeg -i input -i logo1 -i logo2 -filter_complex 'overlay=x=10:y=H-h-10,overlay=x=W-w-10:y=H-h-10' output
//
// ffmpeg -i title_t03.mkv -vn -an -scodec xsub -f image2 out%03d.xsub
//
//
// Pilko faili palasiksi jo ennen croppia ja tarkista sitten kaikista palasista kroppiarvot.
// Splittaus käyttää flac audiota ja siksi pakottaa wrapperiksi mkv:n muista kirjata helppeihin
//
// Onks subtitle horizontal offset jo tehty ? Tsekkaa overlay - komentoja alta Avengers3:sta
// Tsekkaa pitäiskö aina --filter_complexin kanssa käyttää audiossa oletus-delaytä (ehkä 40 ms).
// Muuta nykyinen subtitle scale optio (-sd), joksin muuksi, esim. -scr (subtitle crop resize) tai -sca (subtitle crop adjust). Sitten tee uudet optiot: -ssd (subtitle scale down) -ssu (subtitle scale up), -shc (subtitle horizontal center). muuta nykyinen optio -so optioksi -svo (subtitle vertical offset) ja tee uusi optio: -sho (subtitle horizontal offset).
// Tee -an optio, joka pakkaa ainoastaan videon ja jättää audio kokonaan pois.
// Pitäiskö tehdä mahdollisuus muxata kohdetiedostoon useamman kielinen tekstitys ? Tässä tulis kohtuullisen isoja koodimuutoksia.
//
// Nimeä ffmpeg_enkooderi uudella nimellä (sl_encoder = starlight encoder) ja poista hakemisto: 00-vanhat jotta git repon voi julkaista
//
// Mites dts-hd ?:
//
// Tämä muuntaa Blurayn pgs - subtitlen dbdsub:iksi (laatu heikkenee jonkin verran):
// ffmpeg -y -threads auto -fix_sub_duration -analyzeduration 20E6 -i extra-S11-02.mkv -scodec dvdsub -map 0:s:0 -map 0:v:0 -vf idet,yadif=0:deint=all -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -f mp4  testi.mp4
// Kun teet PGS ---> DVDSUB muunnosoption, muista muuttaa option -sm help - teksti.
// Bluray subtitleja (hdmv_pgs_subtitle) ei voi ympätä mp4 failiin, ne on tuettuja ilmeisesti vaan mkv ja .ts wrappereissa. Tsekkaa toimiiko alla olevassa linkissä oleva hdmv_pgs_subtitle muunnos dvd_subtitleksi.
// https://trac.ffmpeg.org/ticket/1277
// https://en.wikibooks.org/wiki/FFMPEG_An_Intermediate_Guide/subtitle_options
// subtitle_palette pitää toimia vain subtitlen tyypeillä dvdsub ja dvbsub.
//
//
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+140[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 1 /dev/null
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+70[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 00:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=(main_w-overlay_w)/2:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 01:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=((main_w-overlay_w)/2)+30:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 1 /dev/null
// ffmpeg -y -loglevel 8 -threads auto -i Avengers-3-Infinity_War.mkv -ss 01:05 -t 01:30 -filter_complex '[0:s:5]scale=w=iw/1.5:h=ih/1.5[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all,crop=1920:800:0:140[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=((main_w-overlay_w)/2)+30:main_h-overlay_h+90[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Avengers-3-Infinity_War/00-processed_files/Avengers-3-Infinity_War.mp4

// Jos ohjelmalle annetusta tiedostojoukosta puuttuu yksi failia, ohjelma exitoi eikä käsittele yhtään tiedostoa.
// Jos kroppausarvot on nolla, poista kroppaysoptiot ffmpegin komentoriviltä ?
// Tee enkoodauksen aikainen FFmpegin tulosteen tsekkaus, joka laskee koodauksen aika-arvion ja prosentit siitä kuinka paljon failia on jo käsitelty (fps ?) Tästä on esimerkkiohjelma muistiinpanoissa, mutta se jumittaa n. 90 sekuntia FFmpeg - enkoodauksen alkamisesta.
// Pitäiskö laittaa optio, jolla vois rajoittaa käytettävien prosessorien lukumäärän ?
//
// start           - 00:00:30      alkaa 00:00:00          kesto 00:00:30
// 00:02:00.800    - 01:02:30      alkaa 00:00:30          kesto 01:00:29.200
// 01:03:00        - 01:33:30      alkaa 01:00:59.200      kesto 00:30:30
// 01:40:00        - 02:00:00      alkaa 01:31:29.200
//
// split_times: start,00:00:30,00:02:00.800,01:02:30,01:03:00,01:33:30,01:40:00,02:00:00
// split_times: start|00:00:30|00:02:00.800|01:02:30|01:03:00|01:33:30|01:40:00|02:00:00
// split_times: start/00:00:30/00:02:00.800/01:02:30/01:03:00/01:33:30/01:40:00/02:00:00
// split_times: start*00:00:30*00:02:00.800*01:02:30*01:03:00*01:33:30*01:40:00*02:00:00
// split_times: start-00:00:30-00:02:00.800-01:02:30-01:03:00-01:33:30-01:40:00-02:00:00
//
// split_times: start,00:00:30,00:02:00.800,01:02:30,01:03:00,01:33:30,01:40:00,02:00:00
// cut_list_positions_and_durations_seconds: [0 30 120.800 3629.200 3780 1830 6000 1200]
// cut_positions_after_processing_seconds: [0 30 3659.200 5489.200]
// cut_positions_as_timecodes: [00:00:30 01:00:59.200 01:31:29.200]
//
//
// Extract each frame to png: ffmpeg -i testi.mkv subtitlet/$subtitle%03d.png

// Filename: Red_Dwarf-S11-E01-Twentica.mkv
// -----------------------------------------
//
// Commandline options:
// ---------------------
// /opt/ffmpeg_enkooderi -s eng Red_Dwarf-S11-E01-Twentica.mkv Red_Dwarf-S11-E02-Samsara.mkv Red_Dwarf-S11-E03-Give_And_Take.mkv Red_Dwarf-S11-E04-Officer_Rimmer.mkv Red_Dwarf-S11-E05-Krysis.mkv Red_Dwarf-S11-E06-Can_Of_Worm
// s.mkv
//
// FFmpeg Pass 1 Options:
// -----------------------
// ffmpeg -y -loglevel 8 -threads auto -i Red_Dwarf-S11-E01-Twentica.mkv -filter_complex '[0:s:0]copy[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][subtitle_proc
// essing_stream]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipist
// e/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Red_Dwarf/00-processed_files/Red_Dwarf-S11-E01-Twentica -f mp4 -pass 1 /dev/null
//
// FFmpeg Pass 2 Options:
// -----------------------
// ffmpeg -y -loglevel 8 -threads auto -i Red_Dwarf-S11-E01-Twentica.mkv -filter_complex '[0:s:0]copy[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][subtitle_proc
// essing_stream]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 -passlogfile /mounttipist
// e/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Red_Dwarf/00-processed_files/Red_Dwarf-S11-E01-Twentica -f mp4 -pass 2 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Red_Dwarf/00-processed_files
// /Red_Dwarf-S11-E01-Twentica.mp4
//
// Pass 1 took: 21m46.497s
// Pass 2 took: 35m33.132s
// Processing took: 57m19.684s
//
// ########################################################################################################################
//
//
// Extract subtitlestream as video to png per frame: ffmpeg -i testi.mkv -vn -an -filter_complex '[0:s:0]copy[subtitle_processing_stream]' -map '[subtitle_processing_stream]' subtitlet/$subtitle%03d.png
//
// Tee kuvista video mustalla pohjalla: ffmpeg -r 24 -f image2 -i subtitlet/%03d.png -pix_fmt yuv420p -vf fps=24,cropdetect koe.webm
//
// Tee yhdestä kuvasta 10 freimin video ja mittaa siitä kroppiarvot: ffmpeg -r 1/1 -f image2 -i subtitlet/270.png -pix_fmt yuv420p -vf fps=10,cropdetect -f null -
// Sama kuin yllä mutta etsi reunat jokaisessa fremissä uudestaan: ffmpeg -r 1/1 -f image2 -i subtitlet/290.png -pix_fmt yuv420p -vf fps=10,cropdetect=24:1:0 -f null -
//
// Freimi 270: lyhyt teksti ruudun yläosassa: crop=-1904:-1072:1914:1078
// Freimi 285: tyhjä feimi, pelkkää läpinäkyvyyttä: crop=-1904:-1072:1914:1078
// Freimi 290: pitkä teksti alhaalla: crop=-1904:112:1914:848
//
// Yllä korppiarvoissa on ongelmana se, että jostain syystä FFmpeg tulostaa negatiivisia arvoja, mutta hyväksyy vain positiivisia. Lisäksi ylä- alasuunnassa tunnistettu raja leikkaa tekstiä, ylärajaa pitää nostaa ja alarajaa laskea 20 pistettä.
//
// Tämä tuottaa freimille 290 oikean kroppauksen: ffmpeg -r 1/25 -f image2 -i subtitlet/290.png -pix_fmt yuv420p -vf fps=10,crop=1912:152:1918:828 koe2.webm
//
// ########################################################################################################################
//
// Tämä löytää paremmin reunat: ffmpeg -r 1/1 -f image2 -i subtitlet/270.png -vf fps=10,cropdetect=1:1:1 -f null -
//
// Freimi 270: lyhyt teksti ruudun yläosassa: crop=192:32:864:108
// Freimi 285: tyhjä feimi, pelkkää läpinäkyvyyttä: crop=-1904:-1072:1914:1078
// Freimi 290: pitkä teksti alhaalla: crop=976:128:464:836
//
// ffmpeg -r 1/5 -f image2 -i subtitlet/270.png -vf "fps=10,format=rgba,geq='r=if(gt(alpha(X,Y),128),255,0):g=if(gt(alpha(X,Y),128),255,0):b=if(gt(alpha(X,Y),128),255,0):a=if(gt(alpha(X,Y),250),255,0)',cropdetect=1:1:1" koe2.webm
//
// Tää taitaa olla paras rivi, mutta ylös ja alas pitää lisätä 20 ja reunoille 10 pixeliä: ffmpeg -r 1/1 -f image2 -i subtitlet/270.png -vf "fps=10,format=rgba,geq='r=if(gt(alpha(X,Y),0),255):g=if(gt(alpha(X,Y),0),255):b=if(gt(alpha(X,Y),0),255):a=0',cropdetect=1:2:1" -f null -
// Yllä oleva tuottaa rivin: crop=200:46:860:102 ja kun lisää ylös ja alas 20 ja laidoille 10 pixeliä toimii tämä rivi hyvin: crop=220:86:850:82
//
// If lauseet toimii näin: jokaisella värikanavalla (r,g,b, alpha) testataan onko pixelien arvo suurempi kuin nolla ja pixelin arvoksi asetetaan 255, eli gt(alpha(X,Y),0) vertaa kaikkilal X:n ja Y:n koordinaateilla väriarvoa nollaan ja palauttaa 1 alpha(X,Y) on suurempi kuin 0. Tämä edellinen arvo (1 tai 0) on if lauseen evaluation, eli jos numero on 1 palauttaa if lause pilkun jälkeen olevan arvon 255 muussa tapauksessa palauttaa 0. If lauseen palauttama arvo päättyy muuttujiin r,g,b ja alpha.
//
// ImageMagic autocrop: https://www.imagemagick.org/discourse-server/viewtopic.php?t=23613
// convert 270.png -fuzz 28% -trim +repage 270_kropattu.png
//
// Tämäkin toimii: convert 270.png -trim 270_kropattu-2.png
// Jos -trim ei jätä jäljelle mitään, tulee herja:
//
// convert 285.png -trim 285_kropattu.png
// convert: geometry does not contain image `285.png' @ warning/attribute.c/GetImageBoundingBox/240.
//
// Tämä palauttaa arvot, joilla ImageMagic tulee kroppaamaan kuvan, mutta ei tee varsinaista kropppia:
//
// convert 290.png -trim -print "%[fx:w]x%[fx:h]+%[fx:page.x]+%[fx:page.y]" null:
// 1012x140+454+83
//
// Tämä palauttaa kroppiarvot ja kroppaa : convert 290.png -trim -print "%[fx:w]x%[fx:h]+%[fx:page.x]+%[fx:page.y]\n" 290-kropattu.png
// 1012x140+454+832
//
// mogrify -trim -print "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]\n" koe.png
// 720,576,457,66,132,429
//
// convert -trim -print "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]\n" koe.png kropattu.png
// 720,576,457,66,132,429
//
// convert 290.png -trim -print "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]\n" 290-kropattu.png
// KOE=`convert 290.png -trim -print "%[W],%[H],%[fx:w],%[fx:h],%[fx:page.x],%[fx:page.y]\n" 290-kropattu.png`
// echo $KOE
//
// 1920,1080,1012,140,454,832
//
// 1920 = Alkuperäisen kuvan leveys
// 1080 = Alkuperäisen kuvan korkeus
// 1012 = Kropatun kuvan leveys
// 140 = Kropatun kuvan korkeus
// 454 = Kroppauksen alkupaikka x akselilla (vasemmasta laidasta laskien ?)
// 832 = Kroppauksen alkupaikka y akselilla ylhäältä laskien (Tämän perusteella voi keskittää tekstin yla- alasuunnassa: onko tämä arvo vähemmän vai enemmän kuin 540 (1080 / 2 = 540))
//
//
//
//
// Tämä kroppaa ja tulostaa siitä tietoja: convert -verbose 290.png -trim 290-kropattu.png
// 290.png PNG 1920x1080 1920x1080+0+0 8-bit sRGB 25690B 0.110u 0:00.069
// 290.png=>290-kropattu.png PNG 1920x1080=>1012x140 1920x1080+454+832 8-bit sRGB 25690B 0.070u 0:00.03
//
// Yllä tulosteessa 832 tarkoittaa sitä paljonko kuvan yläreunasta alaspäin on pistettu pixeleitäi, ja 454 taas sitä kuinka paljon vasemmasta laidasta oikealle on poistettu pixeleitä.
// Yllä tulosteessa 1012 tarkoittaa kropatun kuvan leveyttä ja 140 korkeutta.
//
// ffmpeg -i testi.mkv -f image2 -i subtitlet_kropattu/%03d.png -filter_complex '[1:v:0]copy[subtitle_processing_stream];[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 rendattu.mp4
//
// cp subtitlet/* subtitlet_kropattu/
// mogrify -trim subtitlet_kropattu/*.png
// Nykii aina kun subtitle vaihtuu: ffmpeg -i testi.mkv -f image2 -i subtitlet_kropattu/%03d.png -filter_complex '[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][1:v:0]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 rendattu.mp4
//
// Nykii aina kun subtitle vaihtuu: ffmpeg -i testi.mkv -f image2 -i subtitlet_kropattu/%03d.png -filter_complex '[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][1:v:0]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 rendattu.mp4
//
// Keskitetty, Nykii aina kun subtitle vaihtuu: ffmpeg -i testi.mkv -f image2 -i subtitlet_kropattu/%03d.png -filter_complex '[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][1:v:0]overlay=(main_w-overlay_w)/2:main_h-overlay_h+0[processed_combined_streams]' -map [processed_combined_streams] -c:v libx264 -preset medium -profile:v high -level 4.1 -b:v 8000k -acodec copy -map 0:a:0 rendattu.mp4
//
//
// ImageMagick kropatun tekstin keskittäminen läpinäkyvälle 1920x1080 pohjalle:
// ----------------------------------------------------------------------------
// mkdir subtitlet-kropattu
// cp subtitlet/*.png subtitlet-kropattu/
// mogrify -trim subtitlet-kropattu/*.png
//
// identify subtitlet-kropattu/270.png
// subtitlet-kropattu/270.png PNG 202x51 1920x1080+859+98 8-bit Grayscale Gray 3384B 0.000u 0:00.00p
// Eli kropatun kuvan leveys on 202 ja korkeus 51.
//
// (1920 / 2) - (202 / 2) = 859
// 10 = pixeliä yläreunasta alaspäin.
// convert -colorspace gray -size 1920x1080 xc:transparent subtitlet-kropattu/270.png -composite -compose over testi.png
//
// Keskitys yläreunaan:
// --------------------
// convert -colorspace gray -size 1920x1080 xc:transparent subtitlet-kropattu/270.png -geometry +859+10 -composite -compose over testi.png
// tai:
// convert -colorspace gray -size 1920x1080 canvas:transparent subtitlet-kropattu/270.png -geometry +859+10 -composite -compose over testi.png
//
//
// Keskitys alareunaan:
// --------------------
// Vähennetään taustakuvan korkeudesta kropatun kuvan korkeus ja vielä 10 pixeliä lisää jottei teksti ole kiinni alalaidassa:
// 1080 - 10 - 51 = 1019
//
// convert -colorspace gray -size 1920x1080 canvas:transparent subtitlet-kropattu/270.png -geometry +859+1019 -composite -compose over testi-2.png
//
//
// https://video.stackexchange.com/questions/24330/how-to-have-an-overlay-move-to-specific-points-at-specific-frames-using-ffmpeg
// ffmpeg -i C:\src\assets\video\base.mp4 -i C:\card.png -y -filter_complex "[0:v][1:v]overlay=x='if(eq(n,439),300,0)':y='if(eq(n,439),300,0)':enable='eq(n,438)+eq(n,439)'[out]" -map [out] -map 0:a -ss 17 C:\temp\j7kthb0v\composit.mp4
//
// Not really. The filter isn't meant for animation like that. You can simplify it somewhat like this: x='0*eq(n,438)+300*eq(n,439)+X*eq(n,567)+...' – Gyan
// I've been playing around with the variable n, in overlay if I were to do: overlay=x='( 605 + -0.8023952095808383 * n)':y='( 406 + -0.4365269461077843 * n)':enable='between(t,438/25,605/25)', does this mean the n would equal 0 and incremented for every frame it's visible for ? (605 - 438) – Shannon Hochkins
//
//
//
//
//
// https://ffmpeg.org/ffmpeg-utils.html
//
// -filter_script TiedostoNimi  tai   -filter_complex_script TiedostoNimi lataa filtteriasetukset tiedostosta
//
// ffmpeg -y -hide_banner -threads auto -i dvd-testi.mkv -f image2 -i /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Red_Dwarf/subtitletesti/00-processed_files/subtitles/dvd-testi.mkv-fixed_subtitles/subtitle-%10d.png -filter_complex '[0:v:0]idet,yadif=0:deint=all[video_processing_stream];[video_processing_stream][1:v:0]overlay=0:main_h-overlay_h+0[processed_combined_streams]' -map '[processed_combined_streams]' -c:v libx264 -preset medium -profile:v main -level 4.0 -b:v 1600k -acodec copy -map 0:a:0 -f mp4 /mounttipiste/Elokuvat-TV-Ohjelmat-Musiikki/00-tee_h264/rippaukset/Red_Dwarf/subtitletesti/00-processed_files/dvd-testi.mp4
//
// convert -size 720x576 canvas:transparent -colorspace gray -alpha on testi.png
//
//
//
//
