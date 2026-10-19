---
title: SCEE Tutorial: Sound
weight: 12
---

Tutorial Version 1.0 - A quick guide to Net Yaroze programming

In this chapter we will be exploring the wonderful world of sound on the PlayStation. There will be sample code to provide a simple demonstration on how to add MIDI and Samples (sound effects) to your game.

PlayStation Sound Processing Unit

The PlayStation uses ADPCM sound compression, which is approximately 4:1. The SPU is capable of playing 24 sound samples simultaneously and the PlayStation has 512Kb of Ram to store all the sound samples.

Making a sample

Samples are created using any sound editor and then converted to a format in which the PlayStation can read. Using the Net Yaroze sound tools you can convert your samples. See Figure 6.0. The Vabtool is highly recommended for creating your VAG’s.

Figure 6.0

```text
┌───────────────────────────────────────────┬───────────────────────────────────────────┐
│ Sound Tool                                │ Description                               │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Smf2seq.exe                               │ Takes a standard MIDI file (format 1      │
│                                           │ data) and converts it into a SEQ data     │
│                                           │ file.                                     │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Aiff2vag.exe                              │ Converts 16-bit straight PCM data or AIFF │
│                                           │ data into a VAG.                          │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Vabtool.exe                               │ Creates a VAB out of VAG samples.         │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Vabedit.exe                               │ This is a new tool for converting WAV,    │
│                                           │ RAW and VAG files and accurately          │
│                                           │ adjusting sample data to change loop      │
│                                           │ points.                                   │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Wav2Vag2.exe                              │ A looping option is added with this       │
│                                           │ version.                                  │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Wav2Vag.exe                               │ Converts your wav sound file to a         │
│                                           │ PlayStation vag file.                     │
├───────────────────────────────────────────┼───────────────────────────────────────────┤
│ Vabsplit.exe                              │ Splits a VAB into a header and a body.    │
└───────────────────────────────────────────┴───────────────────────────────────────────┘
```

VAB Format

Once you have created your VAG data using the sound tools you then need to make a VAB. A VAB is used as a bank file for sampled data. It contains all the VAG files, which are ADPCM samples. See Figure 6.1.

Figure 6.1

| VAB     | Can only have a maximum of 16 opened at once             |
|---------|----------------------------------------------------------|
| VAB     | Each can contain 254 of VAG data                         |
| VAB     | Each can contain 128 Programs                            |
| Program | Each can contain 16 tones ( maximum 2048 tones per VAB ) |

Getting samples into sound RAM

Once all the sound samples are converted into VAB data you need to split it into two. Using the tool vabsplit.exe, the VAB is then split into a header and a body. The VAB header (.VH) gives you a VABid number and the VAB body (.VB) contains the waveform part. The VAB header and VAB body have to be transferred into main memory first. After the VAB body has been transferred into main memory it is then transferred into the sound RAM. The VAB header must remain in main memory but the VAB body can be removed once it has been transferred into sound ram. See Listing 6.0 for loading a VAB and a SEQ data file. Also see sample program in "chap6" directory.

Listing 6.0

    //--------------------------------------------------------------------------
    // D E F I N E S
    //--------------------------------------------------------------------------

    // sound address
    #define VH_ADDR	 (0x80090000)
    #define VB_ADDR	 (0x800A0000)
    #define SEQ_ADDR (0x80110000)

    // volume defines
    #define MAIN_VOL (127)
    #define SEQ_VOL  (127)

    //--------------------------------------------------------------------------
    // Function: InitSound()
    // Description: Init sound system. Load in vabs and seqs, set reverb and volume
    // Parameters: none
    // Returns: none
    // Notes: N/A
    //--------------------------------------------------------------------------

    void InitSound( void )
     {

     	// return vab_id (0-15), open VAB and transfer to sound buffer
    	vab_id = SsVabTransfer((u_char*)VH_ADDR, (u_char*)VB_ADDR,-1,1);

    	if( vab_id <0 )
    	  {
    	     printf(" vab:%d", vab_id);
    		 printf(" ERROR: VAB fail (LINE:%d)(FILE:%s)\n",
    			      __LINE__,__FILE__);

           }

    	// open SEQ, max of 32 SEQ's can be opened at the same time
    	seq_id = SsSeqOpen((u_long*)SEQ_ADDR,vab_id);

    	if( seq_id <0 )
    	  {
    	    printf(" seq:%d", seq_id);
    	    printf(" ERROR: SEQ fail (LINE:%d)(FILE:%s)\n",
    			     __LINE__,__FILE__);

           }

    	// set reverb
    	SetupReverb();

    	// set main volume
    	SsSetMVol(MAIN_VOL,MAIN_VOL);

    	// set SEQ volume
    	SsSeqSetVol(seq_id,SEQ_VOL,SEQ_VOL);

     }// end InitSound

Playing a sound sample

Use the SsUtKeyOn() routine for playing your sample. This routine takes seven arguments such as the vabid, program number, tone number, note number, fine tune number, volume left, and volume right. Use the Net Yaroze sound tool "vabtool" to check for these attributes. You can play sample by using the following routines in Listing 6.1.

Listing 6.1

    //--------------------------------------------------------------------------
    // Function: InitSFX()
    // Description: Setup our sound effects structure
    // Parameters: Lots...see Library Reference Manual
    // Returns: none
    // Notes: See Library Reference Manual
    //--------------------------------------------------------------------------

    void InitSFX( voice_ptr sound_effect, short vab_id, short program,
                  short tone, short note, short fine, short vol_l, short vol_r )
     {

    	sound_effect-vabid   = vab_id;
    	sound_effect-program = program;
    	sound_effect-tone    = tone;
    	sound_effect-note    = note;
    	sound_effect-fine    = fine;
    	sound_effect-vol_l   = vol_l;
    	sound_effect-vol_r   = vol_r;

     }// end InitSFX

    //--------------------------------------------------------------------------
    // Function: PlaySFX()
    // Description: Play the sound effect
    // Parameters: voice_ptr sound_effect: address of sndfx variable
    // Returns: none
    // Notes: See Library Reference Manual
    //--------------------------------------------------------------------------

    void PlaySFX( voice_ptr sound_effect )
     {

    	 SsUtKeyOn(sound_effect-vabid,
    	       	   sound_effect-program,
    	           sound_effect-tone,
    		   sound_effect-note,
    		   sound_effect-fine,
    	           sound_effect-vol_l,
    	           sound_effect-vol_r);

     }// end PlaySFX

Playing a MIDI sample

After you have converted your MIDI file using the Net Yaroze sound tool "SMF2SEQ" you can then load in into main memory. You must open the SEQ data file by using SsSeqOpen() first. This routine takes the address of the SEQ data file and an ID number of the VAB as its arguments. A Sequence Access Number is returned, and a maximum of 32 SEQ data files can be opened at once. See Listing 6.2 on how to play a SEQ data file.

Listing 6.2

    //--------------------------------------------------------------------------
    // Function: PlayMusic()
    // Description: Start playing the SEQ file.  Will play the SEQ 1 time only!
    // Parameters: none
    // Returns: none
    // Notes: Look at the Library Reference Manual
    //--------------------------------------------------------------------------

    void PlayMusic( void )
     {
    	// read in music and begin playing
    	SsSeqPlay(seq_id, SSPLAY_PLAY, 1);

     }// end PlaySound

Manipulating samples and MIDI files

There are a variety of routines to change the tempo, volume, pitch and etc. Check the Net Yaroze Library book for further information. Reverb can also be used on the PlayStation. The reverb uses the SPU RAM as a buffer, therefore the longer the reverb, the more memory is required. See Figure 6.2. When using the reverb set the type, depth, delay, and the feedback.

If the reverb type is ECHO or DELAY you can’t specify the delay or the feedback. See Listing 6.3 for example.

Figure 6.2

```text
┌────────────────────────────┬────────────────────────────┬─────────────────────────────┐
│ Effect                     │ Memory                     │ Description                 │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Room                       │ 9,920                      │ Short Reverb                │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Studio A                   │ 8,000                      │ Small Studio Reverb         │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Studio B                   │ 18,496                     │ Medium Studio Reverb        │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Studio C                   │ 28,640                     │ Large Studio Reverb         │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Hall                       │ 44,512                     │ Large Hall Reverb           │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Space                      │ 63,368                     │ Huge Spatial Reverb         │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Echo                       │ 98,368                     │ Single Echo                 │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Delay                      │ 98,368                     │ Repeating Delay             │
├────────────────────────────┼────────────────────────────┼─────────────────────────────┤
│ Pipe                       │ 7,072                      │ Metallic Pipe               │
└────────────────────────────┴────────────────────────────┴─────────────────────────────┘
```

Listing 6.3

    //--------------------------------------------------------------------------
    // Function: SetupReverb()
    // Description: Set reverb type, depth, delay and feedback
    // Parameters: none
    // Returns: none
    // Notes: Look at the Library Reference Manual
    //--------------------------------------------------------------------------

    void SetupReverb( void )
     {

    	short type;

    	// set reverb type
    	type = SsUtSetReverbType( SS_REV_TYPE_HALL );

        if( type <0 )
    	    printf(" ERROR: Reverb fail \n");

        // turn on reverb
    	SsUtReverbOn();

        // wait awhile
    	VSync(0);
    	VSync(0);

        // set depth
    	SsUtSetReverbDepth(127,127);

    	// set delay
    	SsUtSetReverbDelay(127);

        // set feedback
    	SsUtSetReverbFeedback(127);

     }// SetupReverb

Closing Down Sound System

When you no longer need any specific SEQ or VAB data files, you just simply stop playing them and close them down. See Listing 6.4.

Listing 6.4

    //--------------------------------------------------------------------------
    // Function: StopSound()
    // Description: Close down the sound system
    // Parameters: none
    // Returns: none
    // Notes: Look at the Library Reference Manual
    //--------------------------------------------------------------------------

    void StopSound( void )
     {

        // stop SEQ data reading
    	SsSeqStop(seq_id);

        // wait awhile
    	VSync(0);
    	VSync(0);

    	// Close VAB data
    	SsVabClose(vab_id);

        // close SEQ data
    	SsSeqClose(seq_id);

     }// end StopSound

Conclusion

Check out the sample programs for further information.
