use std::vec::Vec;

pub fn encode_quote_with_collateral(quote: Vec<u8>, coll: Vec<u8>) -> Vec<u8> {
    let quote_offset = (quote.len() as u32).to_le_bytes();
    let coll_offset = (coll.len() as u32).to_le_bytes();
    
    let mut output = Vec::<u8>::default();
    output.extend(quote_offset);
    output.extend(coll_offset);
    output.extend(quote);
    output.extend(coll);
    
    output
}

pub fn decode_quote_with_collateral(encoded: *const u8, encoded_len: u32) -> (Vec<u8>, Vec<u8>) {
    let mut quote: Vec<u8> = Vec::default();
    let mut collateral: Vec<u8> = Vec::default();
    
    let len_offset = std::mem::size_of::<u32>() as u32 * 2;
    if encoded_len > len_offset {
        // decode quote
        let p_encoded = encoded as *const u32;
        let quote_len = u32::from_le(unsafe { *p_encoded });
        let coll_len = u32::from_le(unsafe { *(p_encoded.offset(1)) });
        
        let total_size = len_offset + quote_len + coll_len;
        if total_size <= encoded_len {
            quote = unsafe {
                std::slice::from_raw_parts(encoded.offset(len_offset as isize), quote_len as usize).to_vec()  
            };
            
            collateral = unsafe {
                std::slice::from_raw_parts(encoded.offset((len_offset + quote_len) as isize), coll_len as usize).to_vec()  
            };   
        }
    }
    
    (quote, collateral)
}