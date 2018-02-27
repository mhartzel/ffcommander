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
type Video_data struct {
	audio_codec string
	sample_rate int
	number_of_channels int
	video_codec string
	picture_width int
	picture_height int
	aspect_ratio string
}

type Crop_values struct {

	// FFmpeg crop values. Cropping first discards "width_offset" amount of pixels from the left side of the picture.
	// Then it discards "height_offset" amount of pixels from the top of the picture.
	// Then cropping takes "picture_width" of pixels to the right and "picture_height" pixels down whats left of the picture.

	picture_width int
	picture_height int
	width_offset int
	height_offset int
}

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

func sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice []string) {

	// Parse ffprobe output, find wrapper, video- and audiostream information in it,
	// and store this info in global maps: complete_stream_info_map and wrapper_info_map.

	var stream_info_str_slice []string
	var text_line_str_slice []string
	var wrapper_info_str_slice []string

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
		// Get media file wrapper information and store it in a slice.
		if strings.HasPrefix(text_line, "format") {
			wrapper_info_str_slice = strings.Split(strings.Replace(text_line, "format.", "", 1), "=")
			wrapper_key := strings.TrimSpace(wrapper_info_str_slice[0])
			wrapper_value := strings.TrimSpace(strings.Replace(wrapper_info_str_slice[1],"\"", "", -1)) // Remove whitespace and " charcters from the data
			wrapper_info_map[wrapper_key] = wrapper_value
		}
	}
}

func get_video_and_audio_stream_information() (Video_data) {

	// Find video and audio stream information and store it as key value pairs in video_stream_info_map and audio_stream_info_map.
	// Discard info about streams that are not audio or video
	var stream_type_is_video bool = false
	var stream_type_is_audio bool = false

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

	// Find specific video and audio info we need and store in a struct that we return to the main program.
	var input_video_info_struct Video_data

	input_video_info_struct.audio_codec = audio_stream_info_map["codec_name"]
	input_video_info_struct.sample_rate,_ = strconv.Atoi(audio_stream_info_map["sample_rate"])
	input_video_info_struct.number_of_channels,_ = strconv.Atoi(audio_stream_info_map["channels"])
	input_video_info_struct.video_codec = video_stream_info_map["codec_name"]
	input_video_info_struct.picture_width,_ = strconv.Atoi(video_stream_info_map["width"])
	input_video_info_struct.picture_height,_ = strconv.Atoi(video_stream_info_map["height"])
	input_video_info_struct.aspect_ratio = video_stream_info_map["display_aspect_ratio"]

	return(input_video_info_struct)
}


func main() {

	// FIXME
	// wrapper_info_mapin tieto kerätään mutta sitä ei käytetä mitenkään.

	/////////////////////////////////////////////////////
	// Test if ffmpeg and ffprobe can be found in path //
	/////////////////////////////////////////////////////
	if _,error := exec.LookPath("ffmpeg") ; error != nil {
		fmt.Println()
		fmt.Println("Error, cant find FFmpeg in path, can't continue.")
		fmt.Println()
		os.Exit(1)
	}

	// Test if ffprobe can be found in path
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
	var audio_stream_number_int = flag.Int("an", 0, "Audio stream number, -an=1 (Use audio stream number 1 in the source file)")
	var grayscale_bool = flag.Bool("gr", false, "Convert video to Grayscale")
	var denoise_bool = flag.Bool("dn", false, "Denoise. Use HQDN3D - filter to remove noise in the picture. Equal to Hanbrake 'medium' noise reduction settings.")
	var force_stereo_bool = flag.Bool("st", false, "Force Audio To Stereo")
	var autocrop_bool = flag.Bool("ac", false, "Autocrop. Find crop values automatically by scanning the star of the file (1800 seconds)")
	var force_hd_bool = flag.Bool("hd", false, "Force Video To HD, Profile = High, Level = 4.1, Bitrate = 8000k")
	var scan_mode_only_bool = flag.Bool("scan", false, "Only scan inputfile and print video and audio stream info.")
	var debug_mode_on = flag.Bool("debug", false, "Turn on debug mode and show info about internal variables.")
	var input_filenames []string
	var crop_values_struct Crop_values

	// Parse commandline options
	flag.Parse()

	// The unparsed options left on the commandline are filenames, store them in a slice.
	for _,file_name := range flag.Args()  {
		input_filenames = append(input_filenames, file_name)
	}

	//////////////////////////////////////////////////
	// Define default processing options for FFmpeg //
	//////////////////////////////////////////////////
	video_compression_options_sd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "main", "-level", "4.0", "-b:v", "1600k"}
	video_compression_options_hd := []string{"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-level", "4.1", "-b:v", "8000k"}
	audio_compression_options := []string{"-acodec", "copy"}
	denoise_options := []string{"hqdn3d=3.0:3.0:2.0:3.0"}

	var deinterlace_options []string

	if *no_deinterlace_bool == true {
		deinterlace_options = []string{"copy"}
	} else {
		deinterlace_options = []string{"idet,yadif=0:deint=interlaced"}
	}
	ffmpeg_commandline_start := []string{"ffmpeg", "-y", "-loglevel", "8", "-threads", "auto"}
	subtitle_number := *subtitle_int

	var grayscale_options []string

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

	var subtitle_processing_options string
	subtitle_options := ""
	output_directory_name := "00-valmiit"
	output_video_format := []string{"-f", "mp4"}
	var ffmpeg_pass_1_commandline []string
	var ffmpeg_pass_2_commandline []string
	var final_crop_string string

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
		fmt.Println("*grayscale_bool:", *grayscale_bool)
		fmt.Println("grayscale_options:",grayscale_options)
		fmt.Println("subtitle_options:",subtitle_options)
		fmt.Println("*autocrop_bool:", *autocrop_bool)
		fmt.Println("*subtitle_int:", *subtitle_int)
		fmt.Println("*no_deinterlace_bool:", *no_deinterlace_bool)
		fmt.Println("*denoise_bool:", *denoise_bool)
		fmt.Println("*force_stereo_bool:", *force_stereo_bool)
		fmt.Println("*force_hd_bool:", *force_hd_bool)
		fmt.Println("*audio_stream_number_int:", *audio_stream_number_int)
		fmt.Println("*scan_mode_only_bool", *scan_mode_only_bool)
		fmt.Println("*debug_mode_on", *debug_mode_on)
		fmt.Println()
		fmt.Println("input_filenames:", input_filenames)
		fmt.Println("\n")
}

	/////////////////////////////////////////
	// Main loop that processess all files //
	/////////////////////////////////////////
	for _,file_name := range input_filenames {

		var command_to_run_str_slice []string

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
		}

		// If output directory does not exist in the input path then create it.
		if _, err := os.Stat(filepath.Join(inputfile_path, output_directory_name)); os.IsNotExist(err) {
			os.Mkdir(filepath.Join(inputfile_path, output_directory_name), 0777)
		}

		// Get video info with: ffprobe -loglevel 16 -show_entries format:stream -print_format flat -i InputFile
		command_to_run_str_slice = append(command_to_run_str_slice, "ffprobe","-loglevel","16","-show_entries","format:stream","-print_format","flat","-i")
		command_to_run_str_slice = append(command_to_run_str_slice, file_name)

		unsorted_ffprobe_information_str_slice, error_message := run_external_command(command_to_run_str_slice)

		if error_message != nil {
			log.Fatal(error_message)
		}

		// Sort info about video and audio streams in the file to a map
		sort_raw_ffprobe_information(unsorted_ffprobe_information_str_slice)

		// Get specific video and audio stream information
		input_video_info_struct := get_video_and_audio_stream_information()

		/////////////////////////////////////////
		// Print variable values in debug mode //
		/////////////////////////////////////////
		if *debug_mode_on == true {
			fmt.Println("input_video_info_struct.audio_codec:", input_video_info_struct.audio_codec)
			fmt.Println("input_video_info_struct.sample_rate:", input_video_info_struct.sample_rate)
			fmt.Println("input_video_info_struct.number_of_channels:", input_video_info_struct.number_of_channels)
			fmt.Println("input_video_info_struct.video_codec:", input_video_info_struct.video_codec)
			fmt.Println("input_video_info_struct.picture_width:", input_video_info_struct.picture_width)
			fmt.Println("input_video_info_struct.picture_height:", input_video_info_struct.picture_height)
			fmt.Println("input_video_info_struct.aspect_ratio:", input_video_info_struct.aspect_ratio)
			fmt.Println("autocrop_bool:", *autocrop_bool)
			fmt.Println()
		}

		//////////////////////
		// Scan - only mode //
		//////////////////////
		if *scan_mode_only_bool == true {

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

			os.Exit(0)
		}

		/////////////////////////////////////////////////////////////
		// Find out autocrop parameters by scanning the input file //
		/////////////////////////////////////////////////////////////

		if *autocrop_bool == true {

			command_to_run_str_slice = nil
			command_to_run_str_slice = append(command_to_run_str_slice, "ffmpeg","-t","1800","-i",file_name)

			command_to_run_str_slice = append(command_to_run_str_slice, "-f", "matroska", "-sn", "-an", "-filter_complex", "cropdetect=24:16:0", "-y", "-crf", "51", "-preset", "ultrafast", "/dev/null")

			crop_value_counter := 0
			var crop_value_map = make(map[string]int)

			if *debug_mode_on == true {
				fmt.Println()
				fmt.Println("FFmpeg crop command:", command_to_run_str_slice)
				fmt.Println()
			}

			ffmpeg_crop_output, ffmpeg_crop_error := run_external_command(command_to_run_str_slice)

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

				crop_values_struct.picture_width,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[0])
				crop_values_struct.picture_height,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[1])
				crop_values_struct.width_offset,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[2])
				crop_values_struct.height_offset,_ = strconv.Atoi(strings.Split(final_crop_string, ":")[3])

				/////////////////////////////////////////
				// Print variable values in debug mode //
				/////////////////////////////////////////
				if *debug_mode_on == true {

				fmt.Println()
				fmt.Println("Crop values are:")

				for crop_value := range crop_value_map {
					fmt.Println(crop_value_map[crop_value], "=", crop_value)
					
				}
				fmt.Println()
				fmt.Println("Biggest crop value is", final_crop_string)
				fmt.Println("crop_values_struct.picture_width:", crop_values_struct.picture_width)
				fmt.Println("crop_values_struct.picture_height:", crop_values_struct.picture_height)
				fmt.Println("crop_values_struct.width_offset:", crop_values_struct.width_offset)
				fmt.Println("crop_values_struct.height_offset:", crop_values_struct.height_offset)
				}

			} else {
				fmt.Println()
				fmt.Println("Scanning inputfile with FFmpeg resulted in an error:")
				fmt.Println(ffmpeg_crop_error)
				fmt.Println()
			}
		}

		/////////////////////////
		// Encode video - mode //
		/////////////////////////
		if *scan_mode_only_bool == false {

			// Create the start of ffmpeg commandline
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, ffmpeg_commandline_start...)
			ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-i", file_name)

			ffmpeg_filter_options := ""

			//////////////////////////////////////////////////////////////////////////////////////////////
			// If there is no subtitle to process use the simple video processing chain (-vf) in FFmpeg //
			// It has only one video input and output.
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
				/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
				// There is a subtitle to process with the video, use the complex video prcessing chain in FFmpeg (-filer_complex) //
				// It can has several isimultaneous video inputs and outputs.
				/////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

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
				if *autocrop_bool == true {
					subtitle_processing_options = "crop=" + strconv.Itoa(crop_values_struct.picture_width) + ":" + strconv.Itoa(crop_values_struct.picture_height) + ":in_w-" + strconv.Itoa(crop_values_struct.picture_width) + ":in_h-" + strconv.Itoa(crop_values_struct.picture_height)
				} else {
					subtitle_processing_options = "copy"
				}

				ffmpeg_pass_2_commandline = append(ffmpeg_pass_2_commandline, "-filter_complex", "[0:s:" + strconv.Itoa(subtitle_number) +
				"]" + subtitle_processing_options + "[subtitle_processing_stream];[0:v:0]" + ffmpeg_filter_options +
				"[video_processing_stream];[video_processing_stream][subtitle_processing_stream]overlay=main_w-overlay_w-0:main_h-overlay_h+" +
				strconv.Itoa(*subtitle_offset_int) + strings.Join(grayscale_options, "") + "[processed_combined_streams]", "-map", "[processed_combined_streams]")
			}

			///////////////////////////////////////////////////////////////////
			// Add video and audio compressing options to FFmpeg commandline //
			///////////////////////////////////////////////////////////////////

			// If video horizontal resolution is over 700 pixel choose HD video compression settings
			video_compression_options := video_compression_options_sd

			if *force_hd_bool || input_video_info_struct.picture_width > 700 {
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

			fmt.Println()
			fmt.Println("ffmpeg_pass_1_commandline:", ffmpeg_pass_1_commandline)
			ffmpeg_pass_1_output, ffmpeg_pass_1_error := run_external_command(ffmpeg_pass_1_commandline)
			fmt.Println()

			if ffmpeg_pass_1_output != nil {
				fmt.Println(ffmpeg_pass_1_output)
			}

			if ffmpeg_pass_1_error != nil {
				fmt.Println(ffmpeg_pass_1_error)
			}

			fmt.Println()
			fmt.Println("ffmpeg_pass_2_commandline:", ffmpeg_pass_2_commandline)
			ffmpeg_pass_2_output, ffmpeg_pass_2_error :=  run_external_command(ffmpeg_pass_2_commandline)
			fmt.Println()

			if ffmpeg_pass_2_output != nil {
				fmt.Println(ffmpeg_pass_2_output)
			}

			if ffmpeg_pass_2_error != nil {
				fmt.Println(ffmpeg_pass_2_error)
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
// Tee -ss ja -t optiot, jotka ohjataan sellaisenaan FFmpegille. Näillä voi testata asetuksia osaan tiedostosta.
// Tee skannaus joka näyttää valitut tiedot lähdetiedostosta: reso, framerate, audioden ja subtitlejen lukumäärä, jne.


