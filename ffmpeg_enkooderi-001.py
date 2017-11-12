#!/usr/bin/env python3                                                                                                                                                                                            
# -*- coding: utf-8 -*-
#
# Copyright (C) Mikael Hartzell 2016
#
# This program is distributed under the GNU General Public License, version 3 (GPLv3)
#
# Check the license here: http://www.gnu.org/licenses/gpl.txt
# Basically this license gives you full freedom to do what ever you wan't with this program. You are free to use, modify, distribute it any way you like.
# The only restriction is that if you make derivate works of this program AND distribute those, the derivate works must also be licensed under GPL 3.
# This program is distributed in the hope that it will be useful, but WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License for more details.
#

import os
import sys
import subprocess
import tempfile

def run_external_command(commands_to_run):

	directory_for_os_temporary_files = tempfile.gettempdir()
	stdout = b''
	stderr = b''
	error_happened = False

	try:
		# Define filenames for temporary files that we are going to use as stdout and stderr for the external command.
		stdout_for_external_command = directory_for_os_temporary_files + os.sep + 'external_command_stdout.txt'
		stderr_for_external_command = directory_for_os_temporary_files + os.sep + 'external_command_stderr.txt'

		# Open the stdout and stderr temporary files in binary write mode.
		with open(stdout_for_external_command, 'wb') as stdout_commandfile_handler, open(stderr_for_external_command, 'wb') as stderr_commandfile_handler:

			# Run our command.
			subprocess.Popen(commands_to_run, stdout=stdout_commandfile_handler, stderr=stderr_commandfile_handler, stdin=None, close_fds=True).communicate()

			# Make sure all data written to temporary stdout and stderr - files is flushed from the os cache and written to disk.
			stdout_commandfile_handler.flush() # Flushes written data to os cache
			os.fsync(stdout_commandfile_handler.fileno()) # Flushes os cache to disk
			stderr_commandfile_handler.flush() # Flushes written data to os cache
			os.fsync(stderr_commandfile_handler.fileno()) # Flushes os cache to disk

	except IOError as reason_for_error:
		error_message = 'Error writing to stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)
	except OSError as reason_for_error:
		error_message = 'Error writing to stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)

	# Open files we used as stdout and stderr for the external program and read in what the program did output to those files.
	try:
		with open(stdout_for_external_command, 'rb') as stdout_commandfile_handler, open(stderr_for_external_command, 'rb') as stderr_commandfile_handler:
			stdout = stdout_commandfile_handler.read(None)
			stderr = stderr_commandfile_handler.read(None)
	except IOError as reason_for_error:
		error_message = 'Error reading from stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)
	except OSError as reason_for_error:
		error_message = 'Error reading from stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)

	try:
		stdout = str(stdout.decode('UTF-8')).split('\n') # Convert sudo possible error output from binary to UTF-8 text.

	except UnicodeDecodeError:
		# If UTF-8 conversion fails, try conversion with another character map.
		stdout = str(stdout.decode('ISO-8859-15')).split('\n') # Convert sudo possible error output from binary to UTF-8 text.

	try:
		stderr = str(stderr.decode('UTF-8')) # Convert sudo possible error output from binary to UTF-8 text.
	
	except UnicodeDecodeError:
		# If UTF-8 conversion fails, try conversion with another character map.
		stderr = str(stderr.decode('ISO-8859-15')).split('\n') # Convert sudo possible error output from binary to UTF-8 text.

	if stderr != '':
		error_happened = True

	# Delete the temporary stdout and stderr - files
	try:
		os.remove(stdout_for_external_command)
		os.remove(stderr_for_external_command)
	except IOError as reason_for_error:
		error_message = 'Error deleting stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)
	except OSError as reason_for_error:
		error_message = 'Error deleting stdout- or stderr - file when running command: ' + ' '.join(commands_to_run) + '. ' + str(reason_for_error)

	return(stdout, stderr, error_happened)

def sort_raw_ffprobe_information(unsorted_ffprobe_information_list):
	
	# Parse ffprobe output, find wrapper, video- and audiostream information in it,
	# and store this info in stream specific dictionaries.
	# Store all stream dictionaries in lists.

	video_stream_temp_list = []
	audio_stream_temp_list = []

	# Collect information about all strems in the media file.
	# The info is collected to stream specific lists and stored in dictionary: complete_stream_info_dict
	# The stream number is used as the dictionary key when saving info list to dict
	for item in unsorted_ffprobe_information_list:

		stream_number = -1
		temp_info_list = []

		# If there are many programs in the file, then the stream information is listed twice by ffprobe,
		# discard duplicate data.
		if item.startswith('programs.program'):
			continue

		if item.startswith('streams.stream'):
			temp_item_list = item.replace('streams.stream.','').split('.')

			if temp_item_list[0].isnumeric() == True:
				stream_number = int(temp_item_list[0])
			else:
				# Stream number could not be understood, skip the stream
				continue

			# If stream number is -1 then we did not find the stream number, skip the stream
			if stream_number < 0:
				continue

			stream_data = item.replace('streams.stream.' + str(stream_number) + '.', '')

			# Add found stream info line to a list of previous info
			# and store it in a dictionary. The stream nymber acts as the dictionary key.
			if stream_number in complete_stream_info_dict:
				temp_info_list = complete_stream_info_dict[stream_number]

			temp_info_list.append(stream_data)
			complete_stream_info_dict[stream_number] = temp_info_list

		# Get media file wrapper information and store it in a list.
		if item.startswith('format'):
			temp_wrapper_list = item.replace('format.','').split('=')
			wrapper_info_dict[temp_wrapper_list[0].strip()] = temp_wrapper_list[1].replace('"','').strip()

	# Go through the stream info we collected above in dictionary 'complete_stream_info_dict' and
	# find and collect audio and video specific info. Store this info in audio and video specific lists.
	# Discard streams that are not audio or video.

	dictionary_keys_list = list(complete_stream_info_dict)
	dictionary_keys_list.sort()

	for key in dictionary_keys_list:
		stream_info_list = complete_stream_info_dict[key]

		temp_dict = {}

		if 'codec_type="video"' in stream_info_list:

			# # This list of text is info for a video stream. Append the whole list of text into a list.
			# video_stream_temp_list.append(stream_info_list)
			# Go through the audio stream info and split info in key, value pairs in a dictionary.

			for item in stream_info_list:
				temp_list = item.split('=')
				item_key = temp_list[0].strip()
				item_value = temp_list[1].replace('"','').strip()
				temp_dict[item_key] = item_value

			video_stream_temp_list.append(temp_dict)
			continue

		if 'codec_type="audio"' in stream_info_list:
			# # This list of text is info for a audio stream. Append the whole list of text into a list.
			# audio_stream_temp_list.append(stream_info_list)

			for item in stream_info_list:
				temp_list = item.split('=')
				item_key = temp_list[0].strip()
				item_value = temp_list[1].replace('"','').strip()
				temp_dict[item_key] = item_value

			audio_stream_temp_list.append(temp_dict)
			continue

	# Now we have wrapper, video- and audiostream information stored in specific lists.
	# Go through it and split ffprobe info into key, value pairs and store it in stream specific dictionaries
	# Then store stream specific dictionaries in lists.
	# At the end we have one list for video streams and this list contains one dictionary per video stream.
	# In one dictionary we have ffprobe info for the stream as key, value pairs like  'codec_name':'h264'
	# The ffprobe info is processed the same way for audiostreams.

	return(video_stream_temp_list, audio_stream_temp_list)

def find_program_in_os_path(program_name_to_find):

	# Find a program in the operating system path. Returns the full path to the program (search for python3 returns: '/usr/bin/python3').
	program_path = ''
	os_environment_list = os.environ["PATH"].split(os.pathsep)

	for os_path in os_environment_list:
		true_or_false = os.path.exists(os_path + os.sep + program_name_to_find) and os.access(os_path + os.sep + program_name_to_find, os.X_OK) # True if program can be found in the path and it has executable permissions on.
		if true_or_false == True: # Program was found and is executable
			program_path = os_path + os.sep + program_name_to_find
	return(program_path)


###############################
# Main program starts here :) #
###############################

error_happened = False
file_path = sys.argv[1]

complete_stream_info_dict = {}
wrapper_info_dict = {}
video_stream_info_list = []
audio_stream_info_list = []

# Mediafile info is gathered using ffmpeg program 'ffprobe', find it's path.
ffprobe_path = ''
ffprobe_path = find_program_in_os_path('ffprobe')

if ffprobe_path == '':
	print()
	print("Error: could not found 'ffprobe' in path, can not continue.")
	print()
	sys.exit(1)

#for argument in sys.argv[1:]:
#
#	if configfile_found == True:
#		configfile_path = argument
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		configfile_found = False
#		continue		
#
#	if argument.lower() == '-configfile':
#		configfile_found = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-debug_lists_and_dictionaries':
#		debug_lists_and_dictionaries = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-debug_file_processing':
#		debug_file_processing = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#	
#	if argument.lower() == '-debug_all':
#		debug_all = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-save_all_measurement_results_to_a_single_debug_file':
#		save_all_measurement_results_to_a_single_debug_file = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#	
#	if argument.lower() == '-finnish':
#		language = 'fi'
#		finnish = 1
#		english = 0
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-silent':
#		silent = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-force-samplepeak':
#		force_samplepeak = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-force-truepeak':
#		force_truepeak = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-force-no-ffmpeg':
#		force_no_ffmpeg = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue
#
#	if argument.lower() == '-force-quit-when-idle':
#		force_quit_when_idle = True
#		arguments_remaining.pop(arguments_remaining.index(argument))
#		continue

#if (configfile_path == '') and (len(arguments_remaining) > 0):
#	target_path = arguments_remaining[0]
#	arguments_remaining.pop(0)
#
#if len(arguments_remaining) != 0:
#	error_message = 'Error: Unknown arguments on commandline: ' * english + 'Virhe: komentorivillä on tuntemattomia argumentteja: ' * finnish + str(arguments_remaining)
#
#	print()
#	print(error_message)
#	print()
#
#	sys.exit(1)

commands_to_run = ['ffprobe','-loglevel','16','-show_entries','format:stream','-print_format','flat','-i',file_path]

stdout, stderr, error_happened = run_external_command(commands_to_run)

# Parse ffprobe information list.
if error_happened == False:
	video_stream_info_list, audio_stream_info_list = sort_raw_ffprobe_information(stdout)
else:
	print()
	print('Error when reading file with ffprobe:', stderr)
	print()
	sys.exit(1)

print()
print('Wrapper info', 'len(wrapper_info_dict):',len(wrapper_info_dict))
print('---------------------------------------------------------------')

for item in sorted(wrapper_info_dict):
	print(item,'=', wrapper_info_dict[item])

print()
print('Video Stream Info', 'len(video_stream_info_dict):',len(video_stream_info_list))
print('---------------------------------------------------------------')

for info_dict in video_stream_info_list:
	for key in sorted(info_dict):
		print(key,'=',info_dict[key])
	print()

print()
print('Audio Stream Info', 'len(audio_stream_info_dict):',len(audio_stream_info_list))
print('---------------------------------------------------------------')

for info_dict in audio_stream_info_list:
	for key in sorted(info_dict):
		print(key,'=',info_dict[key])
	print()
print()

print()
print('##############################################################################')
print('stderr: ', stderr)
print('##############################################################################')
print()
print('error_happened: ', error_happened)
print()

print('##############################################################################')
print('stdout: ')
print()

#for item in stdout:
#	print(item)

# complete_stream_info_dict_keylist = list(complete_stream_info_dict)
# complete_stream_info_dict_keylist.sort()
# 
# 
# # print('complete_stream_info_dict_keylist:', complete_stream_info_dict_keylist)
# 
# for item in complete_stream_info_dict_keylist:
# 	temp_list = complete_stream_info_dict[item]
# 	print()
# 	print(item)
# 	print('-----')
# 
# 	for list_item in temp_list:
# 		print(list_item)
# 
# print()
# print('##############################################################################')
# 
# 
